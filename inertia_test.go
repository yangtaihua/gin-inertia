package inertia

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestLazyLoading(t *testing.T) {
	props := P{
		"foo":  "bar",
		"lazy": func() interface{} { return "eval" },
	}

	req, _ := http.NewRequest(http.MethodGet, "test", nil)
	req.Header.Add("X-Inertia", "true")
	rw := httptest.NewRecorder()
	c, _ := GetGinContext(rw, req)
	intertiaInstance := Inertia{Version: "test"}
	intertiaInstance.render(c, "events", props)

	resp := rw.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}

	body, _ := ioutil.ReadAll(rw.Result().Body)
	unmarshalledBody := page{}
	err := json.Unmarshal(body, &unmarshalledBody)
	if err != nil {
		t.Error(err)
	}
	sentProps := unmarshalledBody.Props

	if !(len(sentProps) == 2 && sentProps["lazy"] == "eval") {
		t.Errorf("Expected props %v, got %s", props, sentProps)
	}

	//g.Run(":80")
}

func TestPartialReload(t *testing.T) {
	tests := []struct {
		name                   string
		component              string
		partialComponentHeader string
		partialDataHeader      string
		props                  P
		expectedProps          P
	}{
		{
			"Send only requested data",
			"events",
			"events",
			"partial",
			P{
				"foo":     "bar",
				"partial": "data",
			},
			P{
				"partial": "data",
			},
		},
		{
			"Rendered component differs from partial header",
			"notEvents",
			"events",
			"partial",
			P{
				"foo":     "bar",
				"partial": "data",
			},
			P{
				"foo":     "bar",
				"partial": "data",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			req, _ := http.NewRequest(http.MethodGet, "test", nil)
			req.Header.Add("X-Inertia", "true")
			req.Header.Add("X-Inertia-Partial-Data", tc.partialDataHeader)
			req.Header.Add("X-Inertia-Partial-Component", tc.partialComponentHeader)
			rw := httptest.NewRecorder()

			c, _ := GetGinContext(rw, req)
			intertiaInstance := Inertia{Version: "test"}
			intertiaInstance.render(c, tc.component, tc.props)

			resp := rw.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status code 200, got %d", resp.StatusCode)
			}

			body, _ := ioutil.ReadAll(rw.Result().Body)
			unmarshalledBody := page{}
			err := json.Unmarshal(body, &unmarshalledBody)
			if err != nil {
				t.Error(err)
			}
			sentProps := unmarshalledBody.Props

			if len(sentProps) != len(tc.expectedProps) {
				t.Errorf("Expected props %s, got %s", tc.expectedProps, sentProps)
			}

			for k := range tc.expectedProps {
				if tc.expectedProps[k] != sentProps[k] {
					t.Errorf("Expected props %s, got %s", tc.expectedProps, sentProps)
				}
			}

		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		left     P
		right    P
		expected P
	}{
		{
			"no conflicting values",
			P{
				"foo": "bar",
			},
			P{
				"bar": "baz",
			},
			P{
				"foo": "bar",
				"bar": "baz",
			},
		},
		{
			"conflicting values (no map)",
			P{
				"foo": "bar",
			},
			P{
				"foo": "baz",
			},
			P{
				"foo": "baz",
			},
		},
		{
			"conflicting values (map)",
			P{
				"foo": P{"foo": "bar"},
			},
			P{
				"foo": P{"bar": "baz"},
			},
			P{
				"foo": P{
					"foo": "bar",
					"bar": "baz",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			tc.left.merge(tc.right)
			if !reflect.DeepEqual(tc.left, tc.expected) {
				tt.Errorf("expected %v, got %v", tc.expected, tc.left)
			}
		})
	}

}

func TestSharedProps(t *testing.T) {
	tests := []struct {
		name          string
		renderProps   P
		shareProps    map[string]interface{}
		expectedProps P
	}{
		{
			"no shared props",
			P{"foo": "bar"},
			nil,
			P{"foo": "bar"},
		},
		{
			"shared props",
			P{"foo": "bar"},
			map[string]interface{}{"bar": "buzz"},
			P{
				"foo": "bar",
				"bar": "buzz",
			},
		},
		{
			"shared lazy prop with request param",
			P{"foo": "bar"},
			map[string]interface{}{
				"bar": func(r *http.Request) interface{} { return r.Header.Get("X-Test-Value") },
			},
			P{
				"foo": "bar",
				"bar": "test-value",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			//init
			inertia := &Inertia{Version: "test"}
			for k, v := range tc.shareProps {
				inertia.Share(k, v)
			}
			req, _ := http.NewRequest(http.MethodGet, "test", nil)
			req.Header.Add("X-Inertia", "true")
			req.Header.Add("X-Test-Value", "test-value")
			rw := httptest.NewRecorder()
			c, _ := GetGinContext(rw, req)

			inertia.render(c, "test", tc.renderProps)

			resp := rw.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status code 200, got %d", resp.StatusCode)
			}

			body, _ := ioutil.ReadAll(rw.Result().Body)
			unmarshalledBody := page{}
			err := json.Unmarshal(body, &unmarshalledBody)
			if err != nil {
				t.Error(err)
			}
			sentProps := P(unmarshalledBody.Props)

			if !reflect.DeepEqual(sentProps, tc.expectedProps) {
				tt.Errorf("expected %v, got %v", tc.expectedProps, sentProps)
			}
		})
	}
}

func GetGinContext(w http.ResponseWriter, r *http.Request) (*gin.Context, *gin.Engine) {
	c, g := gin.CreateTestContext(w)
	c.Request = r
	return c, g
}
