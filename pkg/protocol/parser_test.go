package protocol

import (
	"testing"
)

func TestParser_ParseCall(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		input   string
		want    *CallRequest
		wantErr bool
	}{
		{
			name: "simple call with params",
			input: `<call name="clean_logs">
  <p>path: /var/log</p>
  <p>days: 7</p>
</call>`,
			want: &CallRequest{
				Name: "clean_logs",
				Params: map[string]string{
					"path": "/var/log",
					"days": "7",
				},
			},
			wantErr: false,
		},
		{
			name: "call with toon data",
			input: `<call name="process_items">
  <p>action: update</p>
  <data type="toon">
items[2]{id,name}:
  1,Apple
  2,Banana
  </data>
</call>`,
			want: &CallRequest{
				Name: "process_items",
				Params: map[string]string{
					"action": "update",
				},
				Data: `items[2]{id,name}:
  1,Apple
  2,Banana`,
			},
			wantErr: false,
		},
		{
			name: "call embedded in text",
			input: `Let me help you clean the logs.
<call name="clean_logs">
  <p>path: /tmp</p>
</call>
Done!`,
			want: &CallRequest{
				Name: "clean_logs",
				Params: map[string]string{
					"path": "/tmp",
				},
			},
			wantErr: false,
		},
		{
			name:    "no call tag",
			input:   "Just some text without any call",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing name attribute",
			input:   `<call><p>param: value</p></call>`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.ParseCall(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCall() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.Name != tt.want.Name {
				t.Errorf("ParseCall() Name = %v, want %v", got.Name, tt.want.Name)
			}

			for k, v := range tt.want.Params {
				if got.Params[k] != v {
					t.Errorf("ParseCall() Params[%s] = %v, want %v", k, got.Params[k], v)
				}
			}

			if tt.want.Data != "" && got.Data != tt.want.Data {
				t.Errorf("ParseCall() Data = %v, want %v", got.Data, tt.want.Data)
			}
		})
	}
}

func TestParser_ParseCalls(t *testing.T) {
	parser := NewParser()

	input := `I'll execute two functions for you.
<call name="func1">
  <p>param: value1</p>
</call>
And then:
<call name="func2">
  <p>param: value2</p>
</call>
All done!`

	calls, err := parser.ParseCalls(input)
	if err != nil {
		t.Fatalf("ParseCalls() error = %v", err)
	}

	if len(calls) != 2 {
		t.Errorf("ParseCalls() got %d calls, want 2", len(calls))
	}

	if calls[0].Name != "func1" {
		t.Errorf("ParseCalls()[0].Name = %v, want func1", calls[0].Name)
	}

	if calls[1].Name != "func2" {
		t.Errorf("ParseCalls()[1].Name = %v, want func2", calls[1].Name)
	}
}

func TestParser_HasCall(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		input string
		want  bool
	}{
		{`<call name="test"></call>`, true},
		{`Some text <call name="test"></call> more text`, true},
		{`No call here`, false},
		{`<call without closing`, false},
		{`</call> without opening`, false},
	}

	for _, tt := range tests {
		got := parser.HasCall(tt.input)
		if got != tt.want {
			t.Errorf("HasCall(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParser_ExtractTextBeforeCall(t *testing.T) {
	parser := NewParser()

	input := `Let me help you with that.
<call name="test"></call>`

	want := "Let me help you with that."
	got := parser.ExtractTextBeforeCall(input)

	if got != want {
		t.Errorf("ExtractTextBeforeCall() = %q, want %q", got, want)
	}
}

func TestParseKeyValue(t *testing.T) {
	tests := []struct {
		input     string
		wantKey   string
		wantValue string
	}{
		{"key: value", "key", "value"},
		{"path: /var/log/app", "path", "/var/log/app"},
		{"number: 42", "number", "42"},
		{"  spaced  :  value  ", "spaced", "value"},
		{"no_colon", "", ""},
		{"url: https://example.com:8080/path", "url", "https://example.com:8080/path"},
	}

	for _, tt := range tests {
		key, value := parseKeyValue(tt.input)
		if key != tt.wantKey || value != tt.wantValue {
			t.Errorf("parseKeyValue(%q) = (%q, %q), want (%q, %q)",
				tt.input, key, value, tt.wantKey, tt.wantValue)
		}
	}
}
