package cli

import (
	"context"
	"fmt"
	"io/ioutil"

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

		opts, err := CreateOptsFromEnv()

		if err != nil {
			panic(err.Error())
		}

		_, err = CreateJob(opts, clientset)

		if err != nil {
			panic(err.Error())
		}
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

	// if image pull secrets are passed in, add them to the pod spec
	if len(opts.ImagePullSecrets) > 0 {
		for _, imagePullSecret := range opts.ImagePullSecrets {
			job.Spec.Template.Spec.ImagePullSecrets = append(job.Spec.Template.Spec.ImagePullSecrets, v1.LocalObjectReference{
				Name: imagePullSecret,
			})
		}
	}

	namespace := job.GetObjectMeta().GetNamespace()

	if namespace == "" {
		namespace = "default"
	}

	return clientset.BatchV1().Jobs(namespace).Create(
		context.Background(),
		job,
		metav1.CreateOptions{},
	)
}
