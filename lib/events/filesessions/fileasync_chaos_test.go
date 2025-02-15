//go:build !race
// +build !race

/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package filesessions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
)

// TestChaosUpload introduces failures in all stages of the async
// upload process and verifies that the system is working correctly.
//
// Data race detector slows down the test significantly (10x+),
// that is why the test is skipped when tests are running with
// `go test -race` flag or `go test -short` flag
func TestChaosUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsC := make(chan events.UploadEvent, 100)
	memUploader := eventstest.NewMemoryUploader(eventsC)
	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:       memUploader,
		MinUploadBytes: 1024,
	})
	require.NoError(t, err)

	scanDir := t.TempDir()
	corruptedDir := t.TempDir()

	var terminateConnection, failCreateAuditStream, failResumeAuditStream atomic.Uint64

	faultyStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
		Inner: streamer,
		OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
			event := pe.GetAuditEvent()
			if event.GetIndex() > 700 && terminateConnection.Add(1) < 5 {
				log.Debugf("Terminating connection at event %v", event.GetIndex())
				return trace.ConnectionProblem(nil, "connection terminated")
			}
			return nil
		},
		OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer events.Streamer) (apievents.Stream, error) {
			if failCreateAuditStream.Add(1) < 5 {
				return nil, trace.ConnectionProblem(nil, "failed to create stream")
			}
			return streamer.CreateAuditStream(ctx, sid)
		},
		OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
			resumed := failResumeAuditStream.Add(1)
			if resumed < 5 {
				// for the first 5 resume attempts, simulate nework failure
				return nil, trace.ConnectionProblem(nil, "failed to resume stream")
			} else if resumed >= 5 && resumed < 8 {
				// for the next several resumes, lose checkpoint file for the stream
				files, err := os.ReadDir(scanDir)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				for _, fi := range files {
					if fi.IsDir() {
						continue
					}
					if fi.Name() == sid.String()+checkpointExt {
						err := os.Remove(filepath.Join(scanDir, fi.Name()))
						require.NoError(t, err)
						log.Debugf("Deleted checkpoint file: %v.", fi.Name())
						break
					}
				}
			}
			return streamer.ResumeAuditStream(ctx, sid, uploadID)
		},
	})
	require.NoError(t, err)

	scanPeriod := 3 * time.Second
	uploader, err := NewUploader(UploaderConfig{
		ScanDir:      scanDir,
		CorruptedDir: corruptedDir,
		ScanPeriod:   scanPeriod,
		Streamer:     faultyStreamer,
		Clock:        clockwork.NewRealClock(),
	})
	require.NoError(t, err)

	go uploader.Serve(ctx)
	defer uploader.Close()

	fileStreamer, err := NewStreamer(scanDir)
	require.NoError(t, err)

	parallelStreams := 20
	type streamState struct {
		sid    string
		events []apievents.AuditEvent
		err    error
	}
	streamsCh := make(chan streamState, parallelStreams)
	for i := 0; i < parallelStreams; i++ {
		go func() {
			inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 4096})
			sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()
			s := streamState{
				sid:    sid,
				events: inEvents,
			}

			stream, err := fileStreamer.CreateAuditStream(ctx, session.ID(sid))
			if err != nil {
				s.err = err
				streamsCh <- s
				return
			}
			for _, event := range inEvents {
				err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
				if err != nil {
					s.err = err
					streamsCh <- s
					return
				}
			}
			s.err = stream.Complete(ctx)
			streamsCh <- s
		}()
	}

	// wait for all streams to be completed
	streams := make(map[string]streamState)
	for i := 0; i < parallelStreams; i++ {
		select {
		case status := <-streamsCh:
			require.NoError(t, status.err)
			streams[status.sid] = status
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for parallel stream complete, try `go test -v` to get more logs for details")
		}
	}

	require.Len(t, streams, parallelStreams)

	for i := 0; i < parallelStreams; i++ {
		select {
		case event := <-eventsC:
			require.NoError(t, event.Error)
			require.Contains(t, streams, event.SessionID, "missing stream for session")

			state := streams[event.SessionID]
			outEvents := readStream(ctx, t, event.UploadID, memUploader)
			require.Equal(t, len(state.events), len(outEvents), fmt.Sprintf("event: %v", event))
		case <-ctx.Done():
			t.Fatal("Timeout waiting for async upload, try `go test -v` to get more logs for details")
		}
	}
}
