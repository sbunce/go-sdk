package statsig

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestLayerExposure(t *testing.T) {
	events := []Event{}

	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			var in *downloadConfigsInput
			bytes, _ := ioutil.ReadFile("layer_exposure_download_config_specs.json")
			json.NewDecoder(req.Body).Decode(&in)
			res.Write(bytes)
		} else if strings.Contains(req.URL.Path, "log_event") {
			type requestInput struct {
				Events          []Event         `json:"events"`
				StatsigMetadata statsigMetadata `json:"statsigMetadata"`
			}
			input := &requestInput{}
			defer req.Body.Close()
			buf := new(bytes.Buffer)
			buf.ReadFrom(req.Body)

			json.Unmarshal(buf.Bytes(), &input)
			events = input.Events
		}
	}))

	opt := &Options{
		API:         testServer.URL,
		Environment: Environment{Tier: "test"},
	}

	user := User{UserID: "some_user_id"}

	start := func() {
		events = []Event{}
		InitializeWithOptions("secret-key", opt)
	}

	//

	t.Run("does not log on getLayer", func(t *testing.T) {
		start()
		GetLayer(user, "unallocated_layer")
		Shutdown()

		if len(events) != 0 {
			t.Errorf("Should receive exactly one log_event")
		}
	})

	//

	t.Run("does not log on non existent keys", func(t *testing.T) {
		start()
		layer := GetLayer(user, "unallocated_layer")
		layer.GetString("a_string", "err")
		Shutdown()

		if len(events) != 0 {
			t.Errorf("Should receive exactly one log_event")
		}
	})

	//

	t.Run("does not log on invalid types", func(t *testing.T) {
		start()
		layer := GetLayer(user, "unallocated_layer")
		layer.GetString("an_int", "err")
		layer.GetBool("an_int", false)
		layer.GetSlice("an_int", make([]interface{}, 0))
		Shutdown()

		if len(events) != 0 {
			t.Errorf("Should receive exactly one log_event")
		}
	})

	//

	t.Run("unallocated layer logging", func(t *testing.T) {
		start()
		layer := GetLayer(user, "unallocated_layer")
		layer.GetNumber("an_int", 0)
		Shutdown()

		if len(events) != 1 {
			t.Errorf("Should receive exactly one log_event")
		}

		if reflect.DeepEqual(events[0].Metadata, map[string]string{
			"config":              "unallocated_layer",
			"ruleID":              "default",
			"allocatedExperiment": "",
			"parameterName":       "an_int",
			"isExplicitParameter": "false",
		}) == false {
			t.Errorf("Invalid metadata")
		}
	})

	//

	t.Run("explicit vs implicit parameter logging", func(t *testing.T) {
		start()
		layer := GetLayer(user, "explicit_vs_implicit_parameter_layer")
		layer.GetNumber("an_int", 0)
		layer.GetString("a_string", "err")
		Shutdown()

		if len(events) != 2 {
			t.Errorf("Should receive exactly two log_events")
		}

		if reflect.DeepEqual(events[0].Metadata, map[string]string{
			"config":              "explicit_vs_implicit_parameter_layer",
			"ruleID":              "alwaysPass",
			"allocatedExperiment": "experiment",
			"parameterName":       "an_int",
			"isExplicitParameter": "true",
		}) == false {
			t.Errorf("Invalid metadata")
		}

		if reflect.DeepEqual(events[1].Metadata, map[string]string{
			"config":              "explicit_vs_implicit_parameter_layer",
			"ruleID":              "alwaysPass",
			"allocatedExperiment": "",
			"parameterName":       "a_string",
			"isExplicitParameter": "false",
		}) == false {
			t.Errorf("Invalid metadata")
		}
	})

	//

	t.Run("logs user and event name", func(t *testing.T) {
		start()
		layer := GetLayer(User{UserID: "dloomb", Email: "d@n.loomb"}, "unallocated_layer")
		layer.GetNumber("an_int", 0)
		Shutdown()

		if len(events) != 1 {
			t.Errorf("Should receive exactly one log_event")
		}

		if events[0].EventName != "statsig::layer_exposure" {
			t.Errorf("Incorrect exposure name")
		}

		if events[0].User.UserID != "dloomb" {
			t.Errorf("Invalid user ID in log")
		}

		if events[0].User.Email != "d@n.loomb" {
			t.Errorf("Invalid email in log")
		}

	})

	defer testServer.Close()

}
