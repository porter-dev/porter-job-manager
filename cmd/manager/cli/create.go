package cli

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
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

		job, err := ReadJobSpec()

		if err != nil {
			panic(err.Error())
		}

		job, err = clientset.BatchV1().Jobs(job.GetObjectMeta().GetNamespace()).Create(
			context.Background(),
			job,
			metav1.CreateOptions{},
		)

		if err != nil {
			panic(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}

func ReadJobSpec() (*batchv1.Job, error) {
	res := &batchv1.Job{}

	bytes, err := ioutil.ReadFile("./job_example.yaml")

	if err != nil {
		return nil, fmt.Errorf("Could not read from file: %s", err.Error())
	}

	err = yaml.Unmarshal(bytes, res)

	if err != nil {
		return nil, fmt.Errorf("Could not parse yaml into job spec: %s", err.Error())
	}

	return res, nil
}
