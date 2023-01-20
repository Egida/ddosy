package ddosy_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	ddosy "github.com/kucicm/ddosy/app"
)

func TestRunLoadTest(t *testing.T) {
	var callCount int32
	done := make(chan struct{})
	testSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&callCount, 1) >= 5 {
			done <- struct{}{}
		}
	}))
	defer testSrv.Close()

	sr := ddosy.ScheduleRequestWeb{
		Endpoint:        testSrv.URL,
		TrafficPatterns: []ddosy.TrafficPatternWeb{{Weight: 1, Payload: "test"}},
		LoadPatterns: []ddosy.LoadPatternWeb{{
			Duration: "1s",
			Linear:   &ddosy.LinearLoadWeb{StartRate: 50, EndRate: 50},
		}},
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(sr)
	req := httptest.NewRequest(http.MethodPost, "/run", &buf)
	w := httptest.NewRecorder()

	cfg := ddosy.ServerConfig{Port: 434343, MaxQueue: 5}
	srv := ddosy.NewServer(cfg)
	srv.ScheduleHandler(w, req)

	res := w.Result()
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("expected error to be nil got %v", err)
	}

	var actual ddosy.ScheduleResponseWeb
	if err := json.Unmarshal(data, &actual); err != nil {
		t.Errorf("falied to unmarshal schedule response %s", err)
	}

	expected := ddosy.ScheduleResponseWeb{Id: 1}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v got %v", expected, actual)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("load test did not start")
	}
}

func TestKillRunningLoadTest(t *testing.T) {
	var callCount int32
	started := make(chan struct{})
	done := make(chan struct{})
	testSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := atomic.AddInt32(&callCount, 1)
		if i == 5 {
			started <- struct{}{}
		}
		if i == 50 {
			// should never reach
			done <- struct{}{}
		}
	}))
	defer testSrv.Close()

	sr := ddosy.ScheduleRequestWeb{
		Endpoint:        testSrv.URL,
		TrafficPatterns: []ddosy.TrafficPatternWeb{{Weight: 1, Payload: "test"}},
		LoadPatterns: []ddosy.LoadPatternWeb{{
			Duration: "10s",
			Linear:   &ddosy.LinearLoadWeb{StartRate: 50, EndRate: 50},
		}},
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(sr)
	req := httptest.NewRequest(http.MethodPost, "/run", &buf)
	w := httptest.NewRecorder()

	cfg := ddosy.ServerConfig{Port: 434343, MaxQueue: 5}
	srv := ddosy.NewServer(cfg)
	srv.ScheduleHandler(w, req)

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Errorf("load test did not start %d\n", callCount)
	}

	req = httptest.NewRequest(http.MethodPost, "/kill", nil)
	w = httptest.NewRecorder()
	srv.KillHandler(w, req)

	select {
	case <-done:
		t.Error("kill did not work")
	case <-time.After(time.Second):
	}
}