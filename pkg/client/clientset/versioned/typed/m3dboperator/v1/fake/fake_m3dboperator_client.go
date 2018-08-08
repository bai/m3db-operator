// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1 "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/typed/m3dboperator/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeOperatorV1 struct {
	*testing.Fake
}

func (c *FakeOperatorV1) M3DBClusters(namespace string) v1.M3DBClusterInterface {
	return &FakeM3DBClusters{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeOperatorV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
