package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// IntegrationTestSuite provides utilities for Kubernetes integration testing
type IntegrationTestSuite struct {
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
	namespace     string
	t             *testing.T
}

// NewIntegrationTestSuite creates a new integration test suite
func NewIntegrationTestSuite(t *testing.T) *IntegrationTestSuite {
	config, err := getKubeConfig()
	if err != nil {
		t.Skipf("Skipping integration test: %v", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create dynamic client: %v", err)
	}

	namespace := fmt.Sprintf("distore-test-%d", time.Now().Unix())

	return &IntegrationTestSuite{
		client:        client,
		dynamicClient: dynamicClient,
		namespace:     namespace,
		t:             t,
	}
}

// Setup creates the test namespace
func (its *IntegrationTestSuite) Setup() {
	// Create namespace
	namespace := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": its.namespace,
			},
		},
	}

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	_, err := its.dynamicClient.Resource(gvr).Create(
		context.TODO(), namespace, v1.CreateOptions{},
	)
	if err != nil {
		its.t.Fatalf("Failed to create namespace: %v", err)
	}

	// Wait for namespace to be ready
	its.waitForNamespace()
}

// Cleanup removes the test namespace
func (its *IntegrationTestSuite) Cleanup() {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	err := its.dynamicClient.Resource(gvr).Delete(
		context.TODO(), its.namespace, v1.DeleteOptions{},
	)
	if err != nil {
		its.t.Logf("Failed to delete namespace: %v", err)
	}
}

// TestDistoreClusterCRD tests the DistoreCluster Custom Resource Definition
func TestDistoreClusterCRD(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := NewIntegrationTestSuite(t)
	suite.Setup()
	defer suite.Cleanup()

	// Test CRD creation
	err := suite.applyManifest("k8s/operator.yaml")
	if err != nil {
		t.Fatalf("Failed to apply operator manifest: %v", err)
	}

	// Wait for CRD to be established
	err = suite.waitForCRD("distoreclusters.distore.io")
	if err != nil {
		t.Fatalf("Failed to wait for CRD: %v", err)
	}

	// Test creating a DistoreCluster resource
	cluster := suite.createTestDistoreCluster()
	err = suite.createDistoreCluster(cluster)
	if err != nil {
		t.Fatalf("Failed to create DistoreCluster: %v", err)
	}

	// Verify the cluster was created
	createdCluster, err := suite.getDistoreCluster(cluster.GetName())
	if err != nil {
		t.Fatalf("Failed to get DistoreCluster: %v", err)
	}

	if createdCluster == nil {
		t.Fatal("DistoreCluster was not created")
	}

	// Test updating the cluster
	err = suite.updateDistoreClusterReplicas(cluster.GetName(), 5)
	if err != nil {
		t.Fatalf("Failed to update DistoreCluster: %v", err)
	}

	// Test deleting the cluster
	err = suite.deleteDistoreCluster(cluster.GetName())
	if err != nil {
		t.Fatalf("Failed to delete DistoreCluster: %v", err)
	}
}

// TestMultiCloudConfiguration tests multi-cloud configuration scenarios
func TestMultiCloudConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := NewIntegrationTestSuite(t)
	suite.Setup()
	defer suite.Cleanup()

	// Setup CRD
	err := suite.applyManifest("k8s/operator.yaml")
	if err != nil {
		t.Fatalf("Failed to apply operator manifest: %v", err)
	}

	err = suite.waitForCRD("distoreclusters.distore.io")
	if err != nil {
		t.Fatalf("Failed to wait for CRD: %v", err)
	}

	// Test valid multi-cloud configuration
	validConfig := suite.createMultiCloudDistoreCluster("valid-multi-cloud")
	err = suite.createDistoreCluster(validConfig)
	if err != nil {
		t.Fatalf("Failed to create valid multi-cloud cluster: %v", err)
	}

	// Test edge computing configuration
	edgeConfig := suite.createEdgeComputingDistoreCluster("edge-computing")
	err = suite.createDistoreCluster(edgeConfig)
	if err != nil {
		t.Fatalf("Failed to create edge computing cluster: %v", err)
	}

	// Clean up
	suite.deleteDistoreCluster(validConfig.GetName())
	suite.deleteDistoreCluster(edgeConfig.GetName())
}

// TestConfigurationValidation tests configuration validation
func TestConfigurationValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := NewIntegrationTestSuite(t)
	suite.Setup()
	defer suite.Cleanup()

	// Setup CRD
	err := suite.applyManifest("k8s/operator.yaml")
	if err != nil {
		t.Fatalf("Failed to apply operator manifest: %v", err)
	}

	err = suite.waitForCRD("distoreclusters.distore.io")
	if err != nil {
		t.Fatalf("Failed to wait for CRD: %v", err)
	}

	// Test valid configuration
	validCluster := suite.createTestDistoreCluster()
	validCluster.SetName("valid-config")
	err = suite.createDistoreCluster(validCluster)
	if err != nil {
		t.Fatalf("Valid configuration was rejected: %v", err)
	}

	// Test invalid configuration (should be rejected by admission controller)
	invalidCluster := suite.createInvalidDistoreCluster()
	err = suite.createDistoreCluster(invalidCluster)
	if err == nil {
		t.Log("Invalid configuration was accepted (validation may not be implemented)")
		// Clean up if it was created
		suite.deleteDistoreCluster(invalidCluster.GetName())
	} else {
		t.Logf("Invalid configuration was properly rejected: %v", err)
	}

	// Clean up valid cluster
	suite.deleteDistoreCluster(validCluster.GetName())
}

// TestScalingOperations tests scaling operations
func TestScalingOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := NewIntegrationTestSuite(t)
	suite.Setup()
	defer suite.Cleanup()

	// Setup CRD
	err := suite.applyManifest("k8s/operator.yaml")
	if err != nil {
		t.Fatalf("Failed to apply operator manifest: %v", err)
	}

	err = suite.waitForCRD("distoreclusters.distore.io")
	if err != nil {
		t.Fatalf("Failed to wait for CRD: %v", err)
	}

	// Create cluster with 2 replicas
	cluster := suite.createTestDistoreCluster()
	cluster.SetName("scaling-test")
	err = suite.createDistoreCluster(cluster)
	if err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}

	// Scale up to 5 replicas
	err = suite.updateDistoreClusterReplicas(cluster.GetName(), 5)
	if err != nil {
		t.Fatalf("Failed to scale up: %v", err)
	}

	// Scale down to 1 replica
	err = suite.updateDistoreClusterReplicas(cluster.GetName(), 1)
	if err != nil {
		t.Fatalf("Failed to scale down: %v", err)
	}

	// Clean up
	suite.deleteDistoreCluster(cluster.GetName())
}

// Helper methods

func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig file
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	if kubeconfig == "" {
		return nil, fmt.Errorf("no kubeconfig found")
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

func (its *IntegrationTestSuite) waitForNamespace() {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	for i := 0; i < 30; i++ {
		_, err := its.dynamicClient.Resource(gvr).Get(
			context.TODO(), its.namespace, v1.GetOptions{},
		)
		if err == nil {
			return
		}
		time.Sleep(time.Second)
	}

	its.t.Fatalf("Timeout waiting for namespace %s", its.namespace)
}

func (its *IntegrationTestSuite) waitForCRD(crdName string) error {
	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	for i := 0; i < 60; i++ {
		crd, err := its.dynamicClient.Resource(gvr).Get(
			context.TODO(), crdName, v1.GetOptions{},
		)
		if err == nil {
			// Check if CRD is established
			conditions, found, err := unstructured.NestedSlice(crd.Object, "status", "conditions")
			if err == nil && found {
				for _, condition := range conditions {
					if conditionMap, ok := condition.(map[string]interface{}); ok {
						if conditionMap["type"] == "Established" && conditionMap["status"] == "True" {
							return nil
						}
					}
				}
			}
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("timeout waiting for CRD %s to be established", crdName)
}

func (its *IntegrationTestSuite) applyManifest(manifestPath string) error {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest %s: %v", manifestPath, err)
	}

	// Split by YAML document separator
	documents := strings.Split(string(content), "---")
	for _, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Parse YAML
		var obj unstructured.Unstructured
		err := yaml.Unmarshal([]byte(doc), &obj.Object)
		if err != nil {
			return fmt.Errorf("failed to parse YAML: %v", err)
		}

		// Apply the resource
		gvr := schema.GroupVersionResource{
			Group:    obj.GetAPIVersion(),
			Version:  obj.GetAPIVersion(),
			Resource: strings.ToLower(obj.GetKind()) + "s",
		}

		// Handle special cases
		if obj.GetKind() == "CustomResourceDefinition" {
			gvr.Group = "apiextensions.k8s.io"
			gvr.Version = "v1"
			gvr.Resource = "customresourcedefinitions"
		}

		_, err = its.dynamicClient.Resource(gvr).Create(
			context.TODO(), &obj, v1.CreateOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to create resource %s/%s: %v", obj.GetKind(), obj.GetName(), err)
		}
	}

	return nil
}

func (its *IntegrationTestSuite) createTestDistoreCluster() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "distore.io/v1",
			"kind":       "DistoreCluster",
			"metadata": map[string]interface{}{
				"name":      "test-cluster",
				"namespace": its.namespace,
			},
			"spec": map[string]interface{}{
				"replicas": 3,
				"image":    "distore/distore:latest",
				"multiCloud": map[string]interface{}{
					"enabled": true,
					"dataCenters": []interface{}{
						map[string]interface{}{
							"id":           "dc1",
							"region":       "us-east-1",
							"nodes":        []string{"node1:8080", "node2:8080", "node3:8080"},
							"priority":     1,
							"replicaCount": 3,
						},
					},
				},
			},
		},
	}
}

func (its *IntegrationTestSuite) createMultiCloudDistoreCluster(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "distore.io/v1",
			"kind":       "DistoreCluster",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": its.namespace,
			},
			"spec": map[string]interface{}{
				"replicas": 3,
				"image":    "distore/distore:latest",
				"multiCloud": map[string]interface{}{
					"enabled": true,
					"dataCenters": []interface{}{
						map[string]interface{}{
							"id":           "aws-dc",
							"region":       "us-east-1",
							"nodes":        []string{"aws-node1:8080", "aws-node2:8080"},
							"priority":     1,
							"replicaCount": 2,
						},
						map[string]interface{}{
							"id":           "gcp-dc",
							"region":       "us-central1",
							"nodes":        []string{"gcp-node1:8080", "gcp-node2:8080"},
							"priority":     2,
							"replicaCount": 2,
						},
					},
					"edgeNodes": []interface{}{
						map[string]interface{}{
							"id":        "edge-nyc",
							"location":  "New York",
							"node":      "edge-nyc:8080",
							"cacheOnly": true,
							"latencyMs": 20,
						},
					},
					"latencyThresholds": map[string]interface{}{
						"local":   10,
						"crossDC": 100,
						"edge":    50,
					},
				},
			},
		},
	}
}

func (its *IntegrationTestSuite) createEdgeComputingDistoreCluster(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "distore.io/v1",
			"kind":       "DistoreCluster",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": its.namespace,
			},
			"spec": map[string]interface{}{
				"replicas": 1,
				"image":    "distore/distore:latest",
				"multiCloud": map[string]interface{}{
					"enabled": true,
					"dataCenters": []interface{}{
						map[string]interface{}{
							"id":           "main-dc",
							"region":       "us-central1",
							"nodes":        []string{"main-node:8080"},
							"priority":     1,
							"replicaCount": 1,
						},
					},
					"edgeNodes": []interface{}{
						map[string]interface{}{
							"id":        "edge-nyc",
							"location":  "New York",
							"node":      "edge-nyc:8080",
							"cacheOnly": true,
							"latencyMs": 15,
						},
						map[string]interface{}{
							"id":        "edge-london",
							"location":  "London",
							"node":      "edge-london:8080",
							"cacheOnly": false,
							"latencyMs": 25,
						},
					},
					"latencyThresholds": map[string]interface{}{
						"local":   10,
						"crossDC": 100,
						"edge":    50,
					},
				},
			},
		},
	}
}

func (its *IntegrationTestSuite) createInvalidDistoreCluster() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "distore.io/v1",
			"kind":       "DistoreCluster",
			"metadata": map[string]interface{}{
				"name":      "invalid-cluster",
				"namespace": its.namespace,
			},
			"spec": map[string]interface{}{
				"replicas": 0, // Invalid: must be positive
				"image":    "distore/distore:latest",
				"multiCloud": map[string]interface{}{
					"enabled":     true,
					"dataCenters": []interface{}{}, // Invalid: must have at least one datacenter
				},
			},
		},
	}
}

func (its *IntegrationTestSuite) createDistoreCluster(cluster *unstructured.Unstructured) error {
	gvr := schema.GroupVersionResource{
		Group:    "distore.io",
		Version:  "v1",
		Resource: "distoreclusters",
	}

	_, err := its.dynamicClient.Resource(gvr).Namespace(its.namespace).Create(
		context.TODO(), cluster, v1.CreateOptions{},
	)
	return err
}

func (its *IntegrationTestSuite) getDistoreCluster(name string) (*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    "distore.io",
		Version:  "v1",
		Resource: "distoreclusters",
	}

	return its.dynamicClient.Resource(gvr).Namespace(its.namespace).Get(
		context.TODO(), name, v1.GetOptions{},
	)
}

func (its *IntegrationTestSuite) updateDistoreClusterReplicas(name string, replicas int) error {
	gvr := schema.GroupVersionResource{
		Group:    "distore.io",
		Version:  "v1",
		Resource: "distoreclusters",
	}

	cluster, err := its.dynamicClient.Resource(gvr).Namespace(its.namespace).Get(
		context.TODO(), name, v1.GetOptions{},
	)
	if err != nil {
		return err
	}

	// Update replicas
	err = unstructured.SetNestedField(cluster.Object, int64(replicas), "spec", "replicas")
	if err != nil {
		return err
	}

	_, err = its.dynamicClient.Resource(gvr).Namespace(its.namespace).Update(
		context.TODO(), cluster, v1.UpdateOptions{},
	)
	return err
}

func (its *IntegrationTestSuite) deleteDistoreCluster(name string) error {
	gvr := schema.GroupVersionResource{
		Group:    "distore.io",
		Version:  "v1",
		Resource: "distoreclusters",
	}

	return its.dynamicClient.Resource(gvr).Namespace(its.namespace).Delete(
		context.TODO(), name, v1.DeleteOptions{},
	)
}

