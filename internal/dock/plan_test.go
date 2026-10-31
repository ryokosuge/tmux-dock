package dock

import (
	"reflect"
	"testing"
)

const testConf = "/path/to/tmux-dock.conf"

func testPlan() Plan {
	return NewPlan(ConfigFromEnv(fakeEnv(map[string]string{})), testConf)
}

func TestNewPlan(t *testing.T) {
	want := Plan{
		HasSession: []string{"-L", "tmux-dock", "has-session", "-t", "=dock"},
		Create: []string{"-L", "tmux-dock", "-f", testConf,
			"new-session", "-d", "-s", "dock", ";", "split-window", "-h", "-t", "=dock:"},
		Attach: []string{"-L", "tmux-dock", "attach-session", "-t", "=dock"},
	}
	if got := testPlan(); !reflect.DeepEqual(got, want) {
		t.Errorf("NewPlan() = %+v, want %+v", got, want)
	}
}

func TestNewPlanSocketIsolation(t *testing.T) {
	cfg := Config{InnerSocket: "inner-x", OuterSocket: "outer-x", SessionName: "dock"}
	plan := NewPlan(cfg, testConf)
	for name, args := range map[string][]string{
		"HasSession": plan.HasSession, "Create": plan.Create, "Attach": plan.Attach,
	} {
		t.Run(name+"は外側のソケットだけを指す", func(t *testing.T) {
			var sockets []string
			for i, a := range args {
				if a == "-L" && i+1 < len(args) {
					sockets = append(sockets, args[i+1])
				}
			}
			if want := []string{"outer-x"}; !reflect.DeepEqual(sockets, want) {
				t.Errorf("%s の -L = %v, want %v (args=%v)", name, sockets, want, args)
			}
			for _, a := range args {
				if a == cfg.InnerSocket {
					t.Errorf("%s が普段のサーバのソケット %q を含む: %v", name, cfg.InnerSocket, args)
				}
			}
		})
	}
}

func TestNewPlanCreateReadsConf(t *testing.T) {
	create := testPlan().Create
	for i, a := range create {
		if a == "-f" && i+1 < len(create) && create[i+1] == testConf {
			return
		}
	}
	t.Errorf("Create に -f %s が無い: %v", testConf, create)
}
