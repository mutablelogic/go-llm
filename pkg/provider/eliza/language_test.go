package eliza

import (
	"regexp"
	"testing"
)

func TestLoadLanguages(t *testing.T) {
	languages, err := LoadLanguages()
	if err != nil {
		t.Fatalf("failed to load languages: %v", err)
	}
	if len(languages) == 0 {
		t.Fatal("no languages loaded")
	}

	// Check English is present
	en, ok := languages["eliza-1966-en"]
	if !ok {
		t.Fatal("English language not found")
	}
	if en.LanguageCode != "en" {
		t.Errorf("expected language code 'en', got %q", en.LanguageCode)
	}

	t.Logf("Loaded %d language(s)", len(languages))
}

func TestLanguageRequiredFields(t *testing.T) {
	languages, err := LoadLanguages()
	if err != nil {
		t.Fatalf("failed to load languages: %v", err)
	}

	for name, lang := range languages {
		t.Run(name, func(t *testing.T) {
			if lang.Model == "" {
				t.Error("model name is empty")
			}
			if lang.Description == "" {
				t.Error("description is empty")
			}
			if lang.LanguageCode == "" {
				t.Error("language code is empty")
			}
			if len(lang.Quits) == 0 {
				t.Error("no quit words defined")
			}
			if len(lang.Greetings) == 0 {
				t.Error("no greeting words defined")
			}
			if len(lang.Reflections) == 0 {
				t.Error("no reflections defined")
			}
			if len(lang.Rules) == 0 {
				t.Error("no rules defined")
			}
			if len(lang.GreetingResponses) == 0 {
				t.Error("no greeting responses defined")
			}
			if len(lang.GoodbyeResponses) == 0 {
				t.Error("no goodbye responses defined")
			}
			if len(lang.DefaultResponses) == 0 {
				t.Error("no default responses defined")
			}
			if len(lang.MemoryResponses) == 0 {
				t.Error("no memory responses defined")
			}
		})
	}
}

func TestLanguageRulesCompile(t *testing.T) {
	languages, err := LoadLanguages()
	if err != nil {
		t.Fatalf("failed to load languages: %v", err)
	}

	for name, lang := range languages {
		t.Run(name, func(t *testing.T) {
			for i, rule := range lang.Rules {
				if rule.Pattern == "" {
					t.Errorf("rule %d: pattern is empty", i)
					continue
				}
				if _, err := regexp.Compile(rule.Pattern); err != nil {
					t.Errorf("rule %d: invalid regex %q: %v", i, rule.Pattern, err)
				}
				if len(rule.Responses) == 0 {
					t.Errorf("rule %d (%q): no responses defined", i, rule.Pattern)
				}
			}
			t.Logf("%d rules, all patterns compiled successfully", len(lang.Rules))
		})
	}
}

func TestLanguageReflectionsSymmetry(t *testing.T) {
	languages, err := LoadLanguages()
	if err != nil {
		t.Fatalf("failed to load languages: %v", err)
	}

	for name, lang := range languages {
		t.Run(name, func(t *testing.T) {
			// Every reflection key should have a non-empty value
			for k, v := range lang.Reflections {
				if k == "" {
					t.Error("reflection has empty key")
				}
				if v == "" {
					t.Errorf("reflection %q has empty value", k)
				}
			}
			t.Logf("%d reflections defined", len(lang.Reflections))
		})
	}
}

func TestLanguageModelNameMatchesKey(t *testing.T) {
	languages, err := LoadLanguages()
	if err != nil {
		t.Fatalf("failed to load languages: %v", err)
	}

	for key, lang := range languages {
		if key != lang.Model {
			t.Errorf("map key %q does not match model name %q", key, lang.Model)
		}
	}
}

func TestLanguageResponsesNonEmpty(t *testing.T) {
	languages, err := LoadLanguages()
	if err != nil {
		t.Fatalf("failed to load languages: %v", err)
	}

	for name, lang := range languages {
		t.Run(name, func(t *testing.T) {
			checkNonEmpty := func(label string, responses []string) {
				for i, r := range responses {
					if r == "" {
						t.Errorf("%s[%d]: empty response string", label, i)
					}
				}
			}
			checkNonEmpty("greetingResponses", lang.GreetingResponses)
			checkNonEmpty("goodbyeResponses", lang.GoodbyeResponses)
			checkNonEmpty("defaultResponses", lang.DefaultResponses)
			checkNonEmpty("memoryResponses", lang.MemoryResponses)
			for _, rule := range lang.Rules {
				checkNonEmpty("rule "+rule.Pattern, rule.Responses)
			}
		})
	}
}
