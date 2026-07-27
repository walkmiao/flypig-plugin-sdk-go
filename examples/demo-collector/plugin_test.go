package main

import (
	"context"
	"testing"

	pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"
	"github.com/walkmiao/flypig-plugin-sdk-go/testkit"
)

func TestPluginAPIConformance(t *testing.T) {
	testkit.RunConformance(t, NewDemoPlugin(), "demo-collector")
}

func TestDemoTaskAndCollect(t *testing.T) {
	plugin := NewDemoPlugin()
	applied, err := plugin.ApplyTasks(context.Background(), &pluginapi.ApplyTasksRequest{Tasks: []*pluginapi.TaskSpec{{
		TaskId: "task-1", Protocol: "demo", Enabled: true, Revision: 1,
	}}})
	if err != nil || len(applied.Results) != 1 || applied.Results[0].Result == nil || !applied.Results[0].Result.Ok {
		t.Fatalf("ApplyTasks response=%+v err=%v", applied, err)
	}
	collected, err := plugin.Collect(context.Background(), &pluginapi.CollectRequest{TaskId: "task-1"})
	if err != nil || collected.Result == nil || !collected.Result.Ok || len(collected.Values) != 2 {
		t.Fatalf("Collect response=%+v err=%v", collected, err)
	}
}

func TestDisabledTaskDoesNotReportRunning(t *testing.T) {
	plugin := NewDemoPlugin()
	applied, err := plugin.ApplyTasks(context.Background(), &pluginapi.ApplyTasksRequest{Tasks: []*pluginapi.TaskSpec{{
		TaskId: "task-disabled", Protocol: "demo", Enabled: false, Revision: 1,
	}}})
	if err != nil || len(applied.Results) != 1 || applied.Results[0].Result == nil || !applied.Results[0].Result.Ok {
		t.Fatalf("ApplyTasks response=%+v err=%v", applied, err)
	}
	listed, err := plugin.ListTasks(context.Background(), &pluginapi.ListTasksRequest{})
	if err != nil || len(listed.Tasks) != 1 {
		t.Fatalf("ListTasks response=%+v err=%v", listed, err)
	}
	task := listed.Tasks[0]
	if task.State != pluginapi.TASK_STATE_STOPPED || task.Connected || task.Ready {
		t.Fatalf("disabled task must remain stopped and disconnected: %+v", task)
	}
}
