package aws

import "testing"

func TestHasGPUNodeGroups(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{
			name: "empty config",
			cfg:  Config{},
			want: false,
		},
		{
			name: "no gpu node groups",
			cfg:  Config{NodeGroups: map[string]NodeGroup{"user": {Instance: "m7i.xlarge"}}},
			want: false,
		},
		{
			name: "mixed cpu and gpu node groups",
			cfg: Config{NodeGroups: map[string]NodeGroup{
				"user": {Instance: "m7i.xlarge"},
				"gpu":  {Instance: "g4dn.2xlarge", GPU: true},
			}},
			want: true,
		},
		{
			name: "only a gpu node group",
			cfg: Config{NodeGroups: map[string]NodeGroup{
				"gpu": {Instance: "g4dn.2xlarge", GPU: true},
			}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.HasGPUNodeGroups(); got != tt.want {
				t.Errorf("HasGPUNodeGroups() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGPUOperatorHelmValues(t *testing.T) {
	v := gpuOperatorHelmValues()

	// On the EKS NVIDIA AMI the driver and toolkit are preinstalled, so the
	// operator only layers on the device plugin.
	componentEnabled := []struct {
		component string
		want      bool
	}{
		{"driver", false},
		{"toolkit", false},
		{"devicePlugin", true},
	}
	for _, tt := range componentEnabled {
		t.Run(tt.component, func(t *testing.T) {
			m, ok := v[tt.component].(map[string]any)
			if !ok {
				t.Fatalf("%s should be a map, got %v", tt.component, v[tt.component])
			}
			if m[enabledKey] != tt.want {
				t.Errorf("%s.enabled = %v, want %v", tt.component, m[enabledKey], tt.want)
			}
		})
	}

	t.Run("MOFED disabled for EFA safety", func(t *testing.T) {
		dp := v["devicePlugin"].(map[string]any)
		env, ok := dp["env"].([]any)
		if !ok || len(env) == 0 {
			t.Fatalf("devicePlugin.env should set MOFED_ENABLED, got %v", dp["env"])
		}
		first, _ := env[0].(map[string]any)
		if first["name"] != "MOFED_ENABLED" || first["value"] != "false" {
			t.Errorf("expected MOFED_ENABLED=false, got %v", first)
		}
	})

	// A GPU node group with a second NoSchedule taint (one team's dedicated
	// pool) must still get the device plugin and NFD, or it never advertises
	// nvidia.com/gpu.
	tolerationPaths := []struct {
		name string
		get  func() any
	}{
		{"daemonsets", func() any { return v["daemonsets"].(map[string]any)["tolerations"] }},
		{"node-feature-discovery.worker", func() any {
			return v["node-feature-discovery"].(map[string]any)["worker"].(map[string]any)["tolerations"]
		}},
	}
	for _, tp := range tolerationPaths {
		t.Run(tp.name+" tolerates every NoSchedule taint", func(t *testing.T) {
			tols, ok := tp.get().([]any)
			if !ok || len(tols) != 1 {
				t.Fatalf("%s.tolerations should hold one toleration, got %v", tp.name, tp.get())
			}
			tol, _ := tols[0].(map[string]any)
			if tol["operator"] != "Exists" || tol["effect"] != "NoSchedule" {
				t.Errorf("%s.tolerations[0] = %v, want operator Exists, effect NoSchedule", tp.name, tol)
			}
			if _, hasKey := tol["key"]; hasKey {
				t.Errorf("%s.tolerations[0] must not set a key, or it tolerates only that taint: %v", tp.name, tol)
			}
		})
	}
}
