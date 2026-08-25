package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tyandl/homelink/internal/config"
	"github.com/tyandl/homelink/internal/frigate"
	"github.com/tyandl/homelink/internal/kasa"
	"github.com/tyandl/homelink/internal/timer"
)

func loadConfig(t *testing.T, toml string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "homelink.toml")
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", path)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func TestBuildRulesMultiServiceFanOut(t *testing.T) {
	cfg := loadConfig(t, `
[[rules]]
name = "mailbox opened"

[rules.trigger]
service = "yolink"
device  = "Mailbox Sensor"
event   = "door.opened"

[[rules.actions]]
service      = "frigate"
device       = "Street Camera"
action       = "create_event"
label        = "mail"
sub_label    = "mailbox"
duration     = 45
bounding_box = [0.1, 0.2, 0.3, 0.4]

[[rules.actions]]
service = "kasa"
device  = "Porch Light"
action  = "turn_on"

[[rules]]
name = "mailbox closed"

[rules.trigger]
service = "yolink"
device  = "Mailbox Sensor"
event   = "door.closed"

[[rules.actions]]
service = "kasa"
device  = "Porch Light"
action  = "set_brightness"
brightness = 40
`)

	rules, usedTriggers, usedActions, err := buildRules(cfg)
	if err != nil {
		t.Fatalf("buildRules: %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
	if !usedTriggers["yolink"] {
		t.Errorf("expected yolink in usedTriggers")
	}
	if !usedActions["frigate"] || !usedActions["kasa"] {
		t.Errorf("expected frigate and kasa in usedActions, got %v", usedActions)
	}

	rule := rules[0]
	if rule.Name != "mailbox opened" || rule.Service != "yolink" || rule.Device != "Mailbox Sensor" || rule.Event != "door.opened" {
		t.Fatalf("unexpected trigger fields: %+v", rule)
	}
	if len(rule.Actions) != 2 {
		t.Fatalf("got %d actions, want 2", len(rule.Actions))
	}

	frigateAction, ok := rule.Actions[0].Config.(*frigate.ActionConfig)
	if !ok {
		t.Fatalf("action 0 config type = %T, want *frigate.ActionConfig", rule.Actions[0].Config)
	}
	if frigateAction.Device != "Street Camera" || frigateAction.Label != "mail" || frigateAction.Duration != 45 {
		t.Errorf("unexpected frigate action: %+v", frigateAction)
	}
	if len(frigateAction.BoundingBox) != 4 || frigateAction.BoundingBox[2] != 0.3 {
		t.Errorf("unexpected bounding box: %v", frigateAction.BoundingBox)
	}

	kasaAction, ok := rule.Actions[1].Config.(*kasa.ActionConfig)
	if !ok {
		t.Fatalf("action 1 config type = %T, want *kasa.ActionConfig", rule.Actions[1].Config)
	}
	if kasaAction.Device != "Porch Light" || kasaAction.Action != "turn_on" {
		t.Errorf("unexpected kasa action: %+v", kasaAction)
	}

	secondRuleAction, ok := rules[1].Actions[0].Config.(*kasa.ActionConfig)
	if !ok {
		t.Fatalf("second rule action config type = %T", rules[1].Actions[0].Config)
	}
	if secondRuleAction.Brightness == nil || *secondRuleAction.Brightness != 40 {
		t.Errorf("unexpected brightness: %+v", secondRuleAction.Brightness)
	}
}

func TestBuildRulesTimerTriggerAndAction(t *testing.T) {
	cfg := loadConfig(t, `
[[rules]]
name = "porch light schedule"

[rules.trigger]
service = "timer"
device  = "afternoon"
event   = "time_of_day"
at      = "16:00"

[[rules.actions]]
service = "kasa"
device  = "Porch Light"
action  = "turn_on"

[[rules]]
name = "mailbox opened"

[rules.trigger]
service = "yolink"
device  = "Mailbox Sensor"
event   = "door.opened"

[[rules.actions]]
service = "timer"
device  = "mailbox lit"
action  = "set"
duration = "5m"

[[rules]]
name = "mailbox light timeout"

[rules.trigger]
service = "timer"
device  = "mailbox lit"
event   = "expired"

[[rules.actions]]
service = "kasa"
device  = "Porch Light"
action  = "turn_off"
`)

	rules, usedTriggers, usedActions, err := buildRules(cfg)
	if err != nil {
		t.Fatalf("buildRules: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rules))
	}
	if !usedTriggers["timer"] {
		t.Errorf("expected timer in usedTriggers, got %v", usedTriggers)
	}
	if !usedActions["timer"] {
		t.Errorf("expected timer in usedActions, got %v", usedActions)
	}

	scheduleRule := rules[0]
	if scheduleRule.Service != "timer" || scheduleRule.Device != "afternoon" || scheduleRule.Event != "time_of_day" {
		t.Fatalf("unexpected schedule rule: %+v", scheduleRule)
	}
	triggerConfig, ok := scheduleRule.TriggerConfig.(*timer.TriggerConfig)
	if !ok {
		t.Fatalf("TriggerConfig type = %T, want *timer.TriggerConfig", scheduleRule.TriggerConfig)
	}
	if triggerConfig.At != "16:00" {
		t.Errorf("unexpected at: %q", triggerConfig.At)
	}

	setAction, ok := rules[1].Actions[0].Config.(*timer.ActionConfig)
	if !ok {
		t.Fatalf("action config type = %T, want *timer.ActionConfig", rules[1].Actions[0].Config)
	}
	if setAction.Device != "mailbox lit" || setAction.Action != "set" || setAction.Duration != "5m" {
		t.Errorf("unexpected set action: %+v", setAction)
	}

	expiredRule := rules[2]
	if expiredRule.Service != "timer" || expiredRule.Device != "mailbox lit" || expiredRule.Event != "expired" {
		t.Errorf("unexpected expired rule: %+v", expiredRule)
	}
}

func TestBuildRulesUnknownTriggerService(t *testing.T) {
	cfg := loadConfig(t, `
[[rules]]
name = "bogus"

[rules.trigger]
service = "not-a-real-service"
device  = "X"
event   = "y"

[[rules.actions]]
service = "kasa"
device  = "Porch Light"
action  = "turn_on"
`)

	if _, _, _, err := buildRules(cfg); err == nil {
		t.Fatal("expected error for unknown trigger service, got nil")
	}
}

func TestBuildRulesUnknownActionService(t *testing.T) {
	cfg := loadConfig(t, `
[[rules]]
name = "bogus"

[rules.trigger]
service = "yolink"
device  = "X"
event   = "door.opened"

[[rules.actions]]
service = "not-a-real-service"
device  = "X"
action  = "turn_on"
`)

	if _, _, _, err := buildRules(cfg); err == nil {
		t.Fatal("expected error for unknown action service, got nil")
	}
}

func TestBuildRulesUnknownYoLinkEvent(t *testing.T) {
	cfg := loadConfig(t, `
[[rules]]
name = "bogus"

[rules.trigger]
service = "yolink"
device  = "X"
event   = "door.explodes"

[[rules.actions]]
service = "kasa"
device  = "Porch Light"
action  = "turn_on"
`)

	if _, _, _, err := buildRules(cfg); err == nil {
		t.Fatal("expected error for unknown yolink event, got nil")
	}
}

func TestBuildRulesRequiresAtLeastOneAction(t *testing.T) {
	cfg := loadConfig(t, `
[[rules]]
name = "no actions"

[rules.trigger]
service = "yolink"
device  = "X"
event   = "door.opened"
`)

	if _, _, _, err := buildRules(cfg); err == nil {
		t.Fatal("expected error for rule with no actions, got nil")
	}
}

func TestBuildRulesInvalidKasaBrightness(t *testing.T) {
	cfg := loadConfig(t, `
[[rules]]
name = "bad brightness"

[rules.trigger]
service = "yolink"
device  = "X"
event   = "door.opened"

[[rules.actions]]
service = "kasa"
device  = "Porch Light"
action  = "set_brightness"
`)

	if _, _, _, err := buildRules(cfg); err == nil {
		t.Fatal("expected error for set_brightness without brightness, got nil")
	}
}
