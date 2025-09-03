package router

import (
	"testing"
)

func TestExactPathMatcher(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		requestPath string
		expected    bool
	}{
		{
			name:        "exact match - root path",
			pattern:     "/",
			requestPath: "/",
			expected:    true,
		},
		{
			name:        "exact match - simple path",
			pattern:     "/login",
			requestPath: "/login",
			expected:    true,
		},
		{
			name:        "exact match - complex path",
			pattern:     "/api/v1/users",
			requestPath: "/api/v1/users",
			expected:    true,
		},
		{
			name:        "no match - different path",
			pattern:     "/login",
			requestPath: "/logout",
			expected:    false,
		},
		{
			name:        "no match - prefix of pattern",
			pattern:     "/login",
			requestPath: "/log",
			expected:    false,
		},
		{
			name:        "no match - pattern is prefix of request",
			pattern:     "/login",
			requestPath: "/login/now",
			expected:    false,
		},
		{
			name:        "no match - case sensitive",
			pattern:     "/Login",
			requestPath: "/login",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewExactPathMatcher(tt.pattern)
			result := matcher.Match(tt.requestPath)
			if result != tt.expected {
				t.Errorf("ExactPathMatcher.Match() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPrefixPathMatcher(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		requestPath string
		expected    bool
	}{
		{
			name:        "prefix match - root",
			prefix:      "/",
			requestPath: "/anything",
			expected:    true,
		},
		{
			name:        "prefix match - api prefix",
			prefix:      "/api",
			requestPath: "/api/users",
			expected:    true,
		},
		{
			name:        "prefix match - exact same",
			prefix:      "/api",
			requestPath: "/api",
			expected:    true,
		},
		{
			name:        "prefix match - with trailing slash",
			prefix:      "/api/",
			requestPath: "/api/users",
			expected:    true,
		},
		{
			name:        "prefix match - deep path",
			prefix:      "/api/v1",
			requestPath: "/api/v1/users/123/profile",
			expected:    true,
		},
		{
			name:        "no match - different prefix",
			prefix:      "/api",
			requestPath: "/web/users",
			expected:    false,
		},
		{
			name:        "no match - partial prefix",
			prefix:      "/api",
			requestPath: "/ap",
			expected:    false,
		},
		{
			name:        "no match - case sensitive",
			prefix:      "/API",
			requestPath: "/api/users",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewPrefixPathMatcher(tt.prefix)
			result := matcher.Match(tt.requestPath)
			if result != tt.expected {
				t.Errorf("PrefixPathMatcher.Match() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRegexPathMatcher(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		requestPath string
		expected    bool
		expectError bool
	}{
		{
			name:        "regex match - simple pattern",
			pattern:     "^/users/[0-9]+$",
			requestPath: "/users/123",
			expected:    true,
		},
		{
			name:        "regex match - complex pattern",
			pattern:     "^/api/v[1-9]/users/[a-zA-Z0-9]+$",
			requestPath: "/api/v1/users/abc123",
			expected:    true,
		},
		{
			name:        "regex match - optional parts",
			pattern:     "^/health(/.*)?$",
			requestPath: "/health",
			expected:    true,
		},
		{
			name:        "regex match - optional parts with path",
			pattern:     "^/health(/.*)?$",
			requestPath: "/health/check",
			expected:    true,
		},
		{
			name:        "no match - pattern mismatch",
			pattern:     "^/users/[0-9]+$",
			requestPath: "/users/abc",
			expected:    false,
		},
		{
			name:        "no match - partial match",
			pattern:     "^/users/[0-9]+$",
			requestPath: "/users/123/profile",
			expected:    false,
		},
		{
			name:        "invalid regex pattern",
			pattern:     "[invalid regex",
			requestPath: "/test",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := NewRegexPathMatcher(tt.pattern)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("NewRegexPathMatcher() expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("NewRegexPathMatcher() unexpected error: %v", err)
				return
			}
			
			result := matcher.Match(tt.requestPath)
			if result != tt.expected {
				t.Errorf("RegexPathMatcher.Match() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPathMatcherFactory(t *testing.T) {
	factory := NewPathMatcherFactory()

	tests := []struct {
		name        string
		rule        PathRule
		requestPath string
		expected    bool
		expectError bool
	}{
		{
			name: "exact matcher creation",
			rule: PathRule{
				Type:  MatchTypeExact,
				Value: "/login",
			},
			requestPath: "/login",
			expected:    true,
		},
		{
			name: "prefix matcher creation",
			rule: PathRule{
				Type:  MatchTypePrefix,
				Value: "/api",
			},
			requestPath: "/api/users",
			expected:    true,
		},
		{
			name: "regex matcher creation",
			rule: PathRule{
				Type:  MatchTypeRegex,
				Value: "^/users/[0-9]+$",
			},
			requestPath: "/users/123",
			expected:    true,
		},
		{
			name: "invalid match type",
			rule: PathRule{
				Type:  "invalid",
				Value: "/test",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := factory.CreateMatcher(tt.rule)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("CreateMatcher() expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("CreateMatcher() unexpected error: %v", err)
				return
			}
			
			result := matcher.Match(tt.requestPath)
			if result != tt.expected {
				t.Errorf("Matcher.Match() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPathMatchEngine(t *testing.T) {
	rules := []PathRule{
		{Type: MatchTypeExact, Value: "/login"},
		{Type: MatchTypePrefix, Value: "/api"},
		{Type: MatchTypeRegex, Value: "^/users/[0-9]+$"},
	}

	engine, err := NewPathMatchEngine(rules)
	if err != nil {
		t.Fatalf("NewPathMatchEngine() error: %v", err)
	}

	tests := []struct {
		name        string
		requestPath string
		expectMatch bool
	}{
		{
			name:        "exact match",
			requestPath: "/login",
			expectMatch: true,
		},
		{
			name:        "prefix match",
			requestPath: "/api/users",
			expectMatch: true,
		},
		{
			name:        "regex match",
			requestPath: "/users/123",
			expectMatch: true,
		},
		{
			name:        "no match",
			requestPath: "/unknown",
			expectMatch: false,
		},
		{
			name:        "partial exact match should fail",
			requestPath: "/login/now",
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Match(tt.requestPath)
			
			if tt.expectMatch {
				if !result.Matched {
					t.Errorf("Match() expected match but got none for path: %s", tt.requestPath)
				}
				if result.MatchedRule == nil {
					t.Errorf("Match() expected MatchedRule but got nil")
				}
			} else {
				if result.Matched {
					t.Errorf("Match() expected no match but got match for path: %s", tt.requestPath)
				}
				if result.MatchedRule != nil {
					t.Errorf("Match() expected no MatchedRule but got: %v", result.MatchedRule)
				}
			}
		})
	}
}

func TestPathMatchEngine_AddRemoveRules(t *testing.T) {
	engine, err := NewPathMatchEngine([]PathRule{})
	if err != nil {
		t.Fatalf("NewPathMatchEngine() error: %v", err)
	}

	// 测试添加规则
	rule := PathRule{Type: MatchTypePrefix, Value: "/test"}
	err = engine.AddRule(rule)
	if err != nil {
		t.Errorf("AddRule() error: %v", err)
	}

	// 验证规则已添加
	result := engine.Match("/test/path")
	if !result.Matched {
		t.Errorf("Match() expected match after adding rule")
	}

	// 测试移除规则
	err = engine.RemoveRule(0)
	if err != nil {
		t.Errorf("RemoveRule() error: %v", err)
	}

	// 验证规则已移除
	result = engine.Match("/test/path")
	if result.Matched {
		t.Errorf("Match() expected no match after removing rule")
	}

	// 测试移除无效索引
	err = engine.RemoveRule(10)
	if err == nil {
		t.Errorf("RemoveRule() expected error for invalid index")
	}
}

func TestPathMatchEngine_MatchAll(t *testing.T) {
	rules := []PathRule{
		{Type: MatchTypePrefix, Value: "/api"},
		{Type: MatchTypePrefix, Value: "/api/v1"},
		{Type: MatchTypeRegex, Value: "^/api/.*$"},
	}

	engine, err := NewPathMatchEngine(rules)
	if err != nil {
		t.Fatalf("NewPathMatchEngine() error: %v", err)
	}

	// 测试匹配多个规则
	results := engine.MatchAll("/api/v1/users")
	
	// 应该匹配所有三个规则
	expectedMatches := 3
	if len(results) != expectedMatches {
		t.Errorf("MatchAll() expected %d matches, got %d", expectedMatches, len(results))
	}

	// 验证所有结果都是匹配的
	for i, result := range results {
		if !result.Matched {
			t.Errorf("MatchAll() result[%d] expected to be matched", i)
		}
		if result.MatchedRule == nil {
			t.Errorf("MatchAll() result[%d] expected MatchedRule", i)
		}
	}
}
