// Copyright 2018 The WPT Dashboard Project. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package receiver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/web-platform-tests/wpt.fyi/api/checks"
	"github.com/web-platform-tests/wpt.fyi/shared"
)

// InternalUsername is a special uploader whose password is kept secret and can
// only be accessed by services in this AppEngine project via Datastore.
const InternalUsername = "_processor"

// HandleResultsCreate handles the POST requests for creating test runs.
func HandleResultsCreate(a AppEngineAPI, s checks.SuitesAPI, w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok || username != InternalUsername || !a.authenticateUploader(username, password) {
		http.Error(w, "Authentication error", http.StatusUnauthorized)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var testRun shared.TestRun
	if err := json.Unmarshal(body, &testRun); err != nil {
		http.Error(w, "Failed to parse JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if testRun.TimeStart.IsZero() {
		testRun.TimeStart = time.Now()
	}
	if testRun.TimeEnd.IsZero() {
		testRun.TimeEnd = testRun.TimeStart
	}
	testRun.CreatedAt = time.Now()

	if len(testRun.FullRevisionHash) != 40 {
		http.Error(w, "full_revision_hash must be the full SHA (40 chars)", http.StatusBadRequest)
		return
	} else if testRun.Revision != "" && strings.Index(testRun.FullRevisionHash, testRun.Revision) != 0 {
		http.Error(w,
			fmt.Sprintf("Mismatch of full_revision_hash and revision fields: %s vs %s", testRun.FullRevisionHash, testRun.Revision),
			http.StatusBadRequest)
		return
	}
	testRun.Revision = testRun.FullRevisionHash[:10]

	key, err := a.addTestRun(&testRun)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.CompleteCheckRun(testRun.FullRevisionHash, testRun.BrowserName)

	// Copy int64 representation of key into TestRun.ID so that clients can
	// inspect/use key value.
	testRun.ID = key.ID

	jsonOutput, err := json.Marshal(testRun)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	w.Write(jsonOutput)
}
