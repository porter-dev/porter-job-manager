package cli

import (
	"context"

	"github.com/joeshaw/envdecode"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var removeAllCmd = &cobra.Command{
	Use: "remove-all",
	Run: func(cmd *cobra.Command, args []string) {
		clientset, err := GetClientset()

		if err != nil {
			panic(err.Error())
		}

		opts, err := RemoveAllOptsFromEnv()

		if err != nil {
			panic(err.Error())
		}

		err = RemoveAllJobs(opts, clientset)

		if err != nil {
			panic(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(removeAllCmd)
}

type RemoveAllOpts struct {
	LabelSelector string `env:"LABEL_SELECTOR"`
	JobNamespace  string `env:"JOB_NAMESPACE"`
}

func RemoveAllOptsFromEnv() (*RemoveAllOpts, error) {
	var c RemoveAllOpts

	if err := envdecode.Decode(&c); err != nil {
		return nil, err
	}

	return &c, nil
}

func RemoveAllJobs(opts *RemoveAllOpts, clientset *kubernetes.Clientset) error {
	namespace := opts.JobNamespace

	if namespace == "" {
		namespace = "default"
	}

	// get all jobs matching label
	matchingJobs, err := clientset.BatchV1().Jobs(namespace).List(
		context.Background(),
		metav1.ListOptions{
			LabelSelector: opts.LabelSelector,
		},
	)

	if err != nil {
		return err
	}

	propPolicy := metav1.DeletePropagationBackground

	for _, job := range matchingJobs.Items {
		err = clientset.BatchV1().Jobs(namespace).Delete(
			context.Background(),
			job.Name,
			metav1.DeleteOptions{
				PropagationPolicy: &propPolicy,
			},
		)

		if err != nil {
			return err
		}
	}

	return nil
}
