package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeTUIBackend struct {
	watchNodes []tuiNode
	readErr    error
}

func (f *fakeTUIBackend) Children(context.Context, tuiNode) ([]tuiNode, error) {
	return []tuiNode{{ID: "child", Label: "Child", ItemName: "Child"}}, nil
}

func (f *fakeTUIBackend) Details(context.Context, tuiNode) ([]tuiAttribute, error) {
	return []tuiAttribute{{Name: "ItemName", Value: "A"}}, nil
}

func (f *fakeTUIBackend) Read(context.Context, tuiNode) (tuiValue, error) {
	if f.readErr != nil {
		return tuiValue{}, f.readErr
	}
	return tuiValue{ID: "A", Label: "A", Value: "42"}, nil
}

func (f *fakeTUIBackend) Watch(ctx context.Context, nodes []tuiNode, _ time.Duration) (<-chan tuiValue, <-chan error, func(), error) {
	f.watchNodes = append([]tuiNode(nil), nodes...)
	values := make(chan tuiValue, 1)
	errs := make(chan error)
	values <- tuiValue{ID: nodes[0].ID, Label: nodes[0].Label, Value: "100"}
	close(values)
	close(errs)
	return values, errs, func() {}, nil
}

func TestTUIControllerLogIsBounded(t *testing.T) {
	c := newTUIController(&fakeTUIBackend{}, time.Second)
	for i := 0; i < maxTUILogLines+5; i++ {
		c.addLog("line")
	}
	if len(c.logs) != maxTUILogLines {
		t.Fatalf("len(logs) = %d, want %d", len(c.logs), maxTUILogLines)
	}
}

func TestTUIControllerMonitorRestartUsesSortedNodes(t *testing.T) {
	backend := &fakeTUIBackend{}
	c := newTUIController(backend, time.Second)
	c.setMonitored(tuiNode{ID: "b", Label: "B"}, tuiValue{ID: "b", Label: "B"})
	c.setMonitored(tuiNode{ID: "a", Label: "A"}, tuiValue{ID: "a", Label: "A"})
	if err := c.restartWatch(context.Background(), func(tuiValue) {}, func(error) {}); err != nil {
		t.Fatalf("restartWatch returned error: %v", err)
	}
	c.stopWatch()
	got := []string{backend.watchNodes[0].ID, backend.watchNodes[1].ID}
	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("watch nodes = %#v, want sorted a,b", got)
	}
}

func TestTUIControllerReadErrorCanBeLogged(t *testing.T) {
	backend := &fakeTUIBackend{readErr: errors.New("boom")}
	c := newTUIController(backend, time.Second)
	if _, err := backend.Read(context.Background(), tuiNode{ID: "A"}); err != nil {
		c.addLog("read A: " + err.Error())
	}
	if !strings.Contains(c.logText(), "read A: boom") {
		t.Fatalf("logText = %q", c.logText())
	}
}
