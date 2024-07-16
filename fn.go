package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	s3v1beta "github.com/upbound/provider-aws/apis/s3/v1beta1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	v1beta1 "github.com/crossplane/function-cnf/input/v1beta1"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-sdk-go/response"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	_ = s3v1beta.AddToScheme(composed.Scheme)
	desired, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get desired resources from %T", req))
		return rsp, nil
	}

	// Generate in.Count number of S3 buckets
	for i := 1; i <= int(in.Count); i++ {
		name := fmt.Sprintf("cnf-bucket-%d", i)
		b := &s3v1beta.Bucket{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"crossplane.io/external-name": name,
				},
			},
			Spec: s3v1beta.BucketSpec{
				ForProvider: s3v1beta.BucketParameters{
					Region: ptr.To[string]("eu-central-1"),
				},
			},
		}
		cd, _ := composed.From(b)
		desired[resource.Name(fmt.Sprintf("xbuckets-%s", name))] = &resource.DesiredComposed{Resource: cd}
	}

	if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composed resources in %T", rsp))
		return rsp, nil
	}

	f.log.Info("Buckets created", "count", in.Count)
	return rsp, nil
}
