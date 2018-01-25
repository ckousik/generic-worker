package main

import (
	"io/ioutil"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/taskcluster/httpbackoff"
)

// Test failure should resolve as "failed"
func TestFailureResolvesAsFailure(t *testing.T) {
	setup(t, "TestFailureResolvesAsFailure")
	defer teardown(t)
	payload := GenericWorkerPayload{
		Command:    failCommand(),
		MaxRunTime: 10,
	}
	td := testTask(t)
	taskID := scheduleAndExecute(t, td, payload)

	ensureResolution(t, taskID, "failed", "failed")
}

func TestAbortAfterMaxRunTime(t *testing.T) {
	setup(t, "TestAbortAfterMaxRunTime")
	defer teardown(t)
	payload := GenericWorkerPayload{
		Command:    sleep(4),
		MaxRunTime: 3,
	}
	td := testTask(t)
	taskID := scheduleAndExecute(t, td, payload)

	ensureResolution(t, taskID, "failed", "failed")
	// check uploaded log mentions abortion
	// note: we do this rather than local log, to check also log got uploaded
	// as failure path requires that task is resolved before logs are uploaded
	url, err := myQueue.GetLatestArtifact_SignedURL(taskID, "public/logs/live_backing.log", 10*time.Minute)
	if err != nil {
		t.Fatalf("Cannot retrieve url for live_backing.log: %v", err)
	}
	resp, _, err := httpbackoff.Get(url.String())
	if err != nil {
		t.Fatalf("Could not download log: %v", err)
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error when trying to read log file over http: %v", err)
	}
	logtext := string(bytes)
	if !strings.Contains(logtext, "max run time exceeded") {
		t.Fatalf("Was expecting log file to mention task abortion, but it doesn't")
	}
	// TODO: this is a hack to make sure sleep process has died before we call teardown
	// We need to make sure processes are properly killed when a task is aborted
	time.Sleep(1500 * time.Millisecond)
}

func TestIdleWithoutCrash(t *testing.T) {
	setup(t, "TestIdleWithoutCrash")
	defer teardown(t)
	if config.ClientID == "" || config.AccessToken == "" {
		t.Skip("Skipping test since TASKCLUSTER_CLIENT_ID and/or TASKCLUSTER_ACCESS_TOKEN env vars not set")
	}
	start := time.Now()
	config.IdleTimeoutSecs = 7
	exitCode := RunWorker()
	end := time.Now()
	if exitCode != IDLE_TIMEOUT {
		t.Fatalf("Was expecting exit code %v, but got exit code %v", IDLE_TIMEOUT, exitCode)
	}
	// Round(0) forces wall time calculation instead of monotonic time in case machine slept etc
	if secsAlive := end.Round(0).Sub(start).Seconds(); secsAlive < 7 {
		t.Fatalf("Worker died early - lasted for %v seconds", secsAlive)
	}
}

func TestRevisionNumberStored(t *testing.T) {
	if !regexp.MustCompile("^[0-9a-f]{40}$").MatchString(revision) {
		t.Fatalf("Git revision could not be determined - got '%v' but expected to match regular expression '^[0-9a-f](40)$'\n"+
			"Did you specify `-ldflags \"-X github.com/taskcluster/generic-worker.revision=<GIT REVISION>\"` in your go test command?\n"+
			"Try using build.sh / build.cmd in root directory of generic-worker source code repository.", revision)
	}
	t.Logf("Git revision successfully retrieved: %v", revision)
}
