package cli

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"sort"

	"github.com/joeshaw/envdecode"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var createCmd = &cobra.Command{
	Use: "create",
	Run: func(cmd *cobra.Command, args []string) {
		clientset, err := GetClientset()

		if err != nil {
			panic(err.Error())
		}

		exitChan := make(chan struct{}, 1)

		opts, err := CreateOptsFromEnv()

		if err != nil {
			panic(err.Error())
		}

		go cleanupJobs(clientset, opts, exitChan)

		_, err = CreateJob(opts, clientset)

		if err != nil {
			panic(err.Error())
		}

		<-exitChan
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}

func ReadJobSpec(bytesPath string) (*batchv1.Job, error) {
	res := &batchv1.Job{}

	bytes, err := ioutil.ReadFile(bytesPath)

	if err != nil {
		return nil, fmt.Errorf("Could not read from file: %s", err.Error())
	}

	err = yaml.Unmarshal(bytes, res)

	if err != nil {
		return nil, fmt.Errorf("Could not parse yaml into job spec: %s", err.Error())
	}

	return res, nil
}

type CreateOpts struct {
	ImagePullSecrets []string `env:"IMAGE_PULL_SECRETS"`
	JobTemplatePath  string   `env:"JOB_TEMPLATE_PATH"`
	LabelSelector    string   `env:"LABEL_SELECTOR"`
	AllowConcurrency bool     `env:"ALLOW_CONCURRENCY"`
}

func CreateOptsFromEnv() (*CreateOpts, error) {
	var c CreateOpts

	if err := envdecode.Decode(&c); err != nil {
		return nil, err
	}

	return &c, nil
}

func CreateJob(opts *CreateOpts, clientset *kubernetes.Clientset) (*batchv1.Job, error) {
	job, err := ReadJobSpec(opts.JobTemplatePath)

	if err != nil {
		return nil, err
	}

	namespace := job.GetObjectMeta().GetNamespace()

	if namespace == "" {
		namespace = "default"
	}

	// If concurrency is not allowed, check for an active Job. If a job is currently active,
	// return without running the job
	if !opts.AllowConcurrency {
		continueVal := ""

		for {
			jobs, err := clientset.BatchV1().Jobs(namespace).List(
				context.Background(),
				metav1.ListOptions{
					LabelSelector: opts.LabelSelector,
					Limit:         25,
					Continue:      continueVal,
				},
			)

			if err != nil {
				return nil, err
			}

			for _, job := range jobs.Items {
				// if any jobs are active, return without error
				if job.Status.Active > 0 && job.Status.Succeeded == 0 {
					return nil, nil
				}
			}

			if jobs.Continue == "" {
				// we have reached the end of the list of jobs
				break
			} else {
				// start pagination
				continueVal = jobs.Continue
			}
		}
	}

	// if image pull secrets are passed in, add them to the pod spec
	if len(opts.ImagePullSecrets) > 0 {
		for _, imagePullSecret := range opts.ImagePullSecrets {
			job.Spec.Template.Spec.ImagePullSecrets = append(job.Spec.Template.Spec.ImagePullSecrets, v1.LocalObjectReference{
				Name: imagePullSecret,
			})
		}
	}

	return clientset.BatchV1().Jobs(namespace).Create(
		context.Background(),
		job,
		metav1.CreateOptions{},
	)
}

func cleanupJobs(clientset *kubernetes.Clientset, opts *CreateOpts, exitChan chan struct{}) {
	log.Println("deleting older job runs, if any")

	job, err := ReadJobSpec(opts.JobTemplatePath)

	if err != nil {
		log.Printf("error reading job spec: %s\n", err.Error())
		exitChan <- struct{}{}
	}

	namespace := job.GetObjectMeta().GetNamespace()

	if namespace == "" {
		namespace = "default"
	}

	var continueStr string
	var failedJobs []batchv1.Job
	var succeededJobs []batchv1.Job

	for {
		jobs, err := clientset.BatchV1().Jobs(namespace).List(
			context.Background(),
			metav1.ListOptions{
				LabelSelector: opts.LabelSelector,
				Continue:      continueStr,
			},
		)

		if err != nil {
			log.Printf("error listing jobs: %s\n", err.Error())
			exitChan <- struct{}{}
		}

		for _, job := range jobs.Items {
			if job.Status.Failed > 0 {
				failedJobs = append(failedJobs, job)
			} else if job.Status.Succeeded > 0 {
				succeededJobs = append(succeededJobs, job)
			}
		}

		if jobs.Continue == "" {
			// we have reached the end of the list of jobs
			break
		} else {
			// start pagination
			continueStr = jobs.Continue
		}
	}

	if len(failedJobs) > 20 {
		sort.SliceStable(failedJobs, func(i, j int) bool {
			return failedJobs[i].Status.CompletionTime.After(failedJobs[j].Status.CompletionTime.Time)
		})

		for _, job := range failedJobs[20:] {
			// we ignore the error in this case because we do not want to fail the job manager
			clientset.BatchV1().Jobs(job.Namespace).Delete(context.Background(), job.Name, metav1.DeleteOptions{})
		}
	}

	if len(succeededJobs) > 20 {
		sort.SliceStable(succeededJobs, func(i, j int) bool {
			return succeededJobs[i].Status.CompletionTime.After(succeededJobs[j].Status.CompletionTime.Time)
		})

		for _, job := range succeededJobs[20:] {
			// we ignore the error in this case because we do not want to fail the job manager
			clientset.BatchV1().Jobs(job.Namespace).Delete(context.Background(), job.Name, metav1.DeleteOptions{})
		}
	}

	log.Println("deleted older job runs, if any")

	exitChan <- struct{}{}
}
