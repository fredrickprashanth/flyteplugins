package container

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"

	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flyteorg/flyteidl/gen/pb-go/flyteidl/core"
	"github.com/flyteorg/flytestdlib/storage"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	pluginsCore "github.com/flyteorg/flyteplugins/go/tasks/pluginmachinery/core"
	pluginsCoreMock "github.com/flyteorg/flyteplugins/go/tasks/pluginmachinery/core/mocks"
	"github.com/flyteorg/flyteplugins/go/tasks/pluginmachinery/flytek8s"
	pluginsIOMock "github.com/flyteorg/flyteplugins/go/tasks/pluginmachinery/io/mocks"
	"github.com/flyteorg/flyteplugins/go/tasks/pluginmachinery/k8s"
)

var resourceRequirements = &v1.ResourceRequirements{
	Limits: v1.ResourceList{
		v1.ResourceCPU:     resource.MustParse("1024m"),
		v1.ResourceStorage: resource.MustParse("100M"),
	},
}

func dummyContainerTaskMetadata(resources *v1.ResourceRequirements) pluginsCore.TaskExecutionMetadata {
	taskMetadata := &pluginsCoreMock.TaskExecutionMetadata{}
	taskMetadata.On("GetNamespace").Return("test-namespace")
	taskMetadata.On("GetAnnotations").Return(map[string]string{"annotation-1": "val1"})
	taskMetadata.On("GetLabels").Return(map[string]string{"label-1": "val1"})
	taskMetadata.On("GetOwnerReference").Return(metav1.OwnerReference{
		Kind: "node",
		Name: "blah",
	})
	taskMetadata.On("GetK8sServiceAccount").Return("service-account")
	taskMetadata.On("GetSecurityContext").Return(core.SecurityContext{
		RunAs: &core.Identity{K8SServiceAccount: "service-account"},
	})
	taskMetadata.On("GetOwnerID").Return(types.NamespacedName{
		Namespace: "test-namespace",
		Name:      "test-owner-name",
	})

	tID := &pluginsCoreMock.TaskExecutionID{}
	tID.On("GetID").Return(core.TaskExecutionIdentifier{
		NodeExecutionId: &core.NodeExecutionIdentifier{
			ExecutionId: &core.WorkflowExecutionIdentifier{
				Name:    "my_name",
				Project: "my_project",
				Domain:  "my_domain",
			},
		},
	})
	tID.On("GetGeneratedName").Return("my_project:my_domain:my_name")
	taskMetadata.On("GetTaskExecutionID").Return(tID)

	to := &pluginsCoreMock.TaskOverrides{}
	to.On("GetResources").Return(nil)
	taskMetadata.On("GetOverrides").Return(to)
	taskMetadata.On("IsInterruptible").Return(true)
	taskMetadata.OnGetResources().Return(resources)
	return taskMetadata
}

func dummyContainerTaskContext(resources *v1.ResourceRequirements, command []string, args []string) pluginsCore.TaskExecutionContext {
	task := &core.TaskTemplate{
		Type: "test",
		Target: &core.TaskTemplate_Container{
			Container: &core.Container{
				Command: command,
				Args:    args,
			},
		},
	}

	dummyTaskMetadata := dummyContainerTaskMetadata(resources)
	taskCtx := &pluginsCoreMock.TaskExecutionContext{}
	inputReader := &pluginsIOMock.InputReader{}
	inputReader.OnGetInputPrefixPath().Return(storage.DataReference("test-data-reference"))
	inputReader.OnGetInputPath().Return(storage.DataReference("test-data-reference"))
	inputReader.OnGetMatch(mock.Anything).Return(&core.LiteralMap{}, nil)
	taskCtx.OnInputReader().Return(inputReader)

	outputReader := &pluginsIOMock.OutputWriter{}
	outputReader.On("GetOutputPath").Return(storage.DataReference("/data/outputs.pb"))
	outputReader.On("GetOutputPrefixPath").Return(storage.DataReference("/data/"))
	outputReader.On("GetRawOutputPrefix").Return(storage.DataReference(""))
	taskCtx.On("OutputWriter").Return(outputReader)

	taskReader := &pluginsCoreMock.TaskReader{}
	taskReader.On("Read", mock.Anything).Return(task, nil)
	taskCtx.On("TaskReader").Return(taskReader)

	taskCtx.On("TaskExecutionMetadata").Return(dummyTaskMetadata)
	return taskCtx
}

func TestContainerTaskExecutor_BuildIdentityResource(t *testing.T) {
	c := Plugin{}
	taskMetadata := &pluginsCoreMock.TaskExecutionMetadata{}
	r, err := c.BuildIdentityResource(context.TODO(), taskMetadata)
	assert.NoError(t, err)
	assert.NotNil(t, r)
	_, ok := r.(*v1.Pod)
	assert.True(t, ok)
	assert.Equal(t, flytek8s.PodKind, r.GetObjectKind().GroupVersionKind().Kind)
}

func TestContainerTaskExecutor_BuildResource(t *testing.T) {
	c := Plugin{}
	command := []string{"command"}
	args := []string{"{{.Input}}"}
	taskCtx := dummyContainerTaskContext(resourceRequirements, command, args)

	r, err := c.BuildResource(context.TODO(), taskCtx)
	assert.NoError(t, err)
	assert.NotNil(t, r)
	j, ok := r.(*v1.Pod)
	assert.True(t, ok)

	assert.NotEmpty(t, j.Spec.Containers)
	assert.Equal(t, resourceRequirements.Limits[v1.ResourceCPU], j.Spec.Containers[0].Resources.Limits[v1.ResourceCPU])

	// TODO: Once configurable, test when setting storage is supported on the cluster vs not.
	storageRes := j.Spec.Containers[0].Resources.Limits[v1.ResourceStorage]
	assert.Equal(t, int64(0), (&storageRes).Value())

	assert.Equal(t, command, j.Spec.Containers[0].Command)
	assert.Equal(t, []string{"test-data-reference"}, j.Spec.Containers[0].Args)

	assert.Equal(t, "service-account", j.Spec.ServiceAccountName)
}

func TestContainerTaskExecutor_GetTaskStatus(t *testing.T) {
	c := Plugin{}
	j := &v1.Pod{
		Status: v1.PodStatus{},
	}

	ctx := context.TODO()
	t.Run("running", func(t *testing.T) {
		j.Status.Phase = v1.PodRunning
		phaseInfo, err := c.GetTaskPhase(ctx, nil, j)
		assert.NoError(t, err)
		assert.Equal(t, pluginsCore.PhaseRunning, phaseInfo.Phase())
	})

	t.Run("queued", func(t *testing.T) {
		j.Status.Phase = v1.PodPending
		phaseInfo, err := c.GetTaskPhase(ctx, nil, j)
		assert.NoError(t, err)
		assert.Equal(t, pluginsCore.PhaseQueued, phaseInfo.Phase())
	})

	t.Run("failNoCondition", func(t *testing.T) {
		j.Status.Phase = v1.PodFailed
		phaseInfo, err := c.GetTaskPhase(ctx, nil, j)
		assert.NoError(t, err)
		assert.Equal(t, pluginsCore.PhaseRetryableFailure, phaseInfo.Phase())
		ec := phaseInfo.Err().GetCode()
		assert.Equal(t, "UnknownError", ec)
	})

	t.Run("failConditionUnschedulable", func(t *testing.T) {
		j.Status.Phase = v1.PodFailed
		j.Status.Reason = "Unschedulable"
		j.Status.Message = "some message"
		j.Status.Conditions = []v1.PodCondition{
			{
				Type: v1.PodReasonUnschedulable,
			},
		}
		phaseInfo, err := c.GetTaskPhase(ctx, nil, j)
		assert.NoError(t, err)
		assert.Equal(t, pluginsCore.PhaseRetryableFailure, phaseInfo.Phase())
		ec := phaseInfo.Err().GetCode()
		assert.Equal(t, "Unschedulable", ec)
	})

	t.Run("success", func(t *testing.T) {
		j.Status.Phase = v1.PodSucceeded
		phaseInfo, err := c.GetTaskPhase(ctx, nil, j)
		assert.NoError(t, err)
		assert.NotNil(t, phaseInfo)
		assert.Equal(t, pluginsCore.PhaseSuccess, phaseInfo.Phase())
	})
}

func TestContainerTaskExecutor_GetProperties(t *testing.T) {
	plugin := Plugin{}
	expected := k8s.PluginProperties{}
	assert.Equal(t, expected, plugin.GetProperties())
}
