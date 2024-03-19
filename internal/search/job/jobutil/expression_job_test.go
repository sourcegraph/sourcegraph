package jobutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/job/mockjob"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type sender struct {
	Job   job.Job
	sendC chan streaming.SearchEvent
}

func (s *sender) Send() {
	res := &result.RepoMatch{Name: "test", ID: 1}
	s.sendC <- streaming.SearchEvent{Results: []result.Match{res}}
}

func (s *sender) Exit() {
	close(s.sendC)
}

type senders []sender

func (ss senders) SendAll() {
	for _, s := range ss {
		s.Send()
	}
}

func (ss senders) ExitAll() {
	for _, s := range ss {
		s.Exit()
	}
}

func (ss senders) Jobs() []job.Job {
	jobs := make([]job.Job, 0, len(ss))
	for _, s := range ss {
		jobs = append(jobs, s.Job)
	}
	return jobs
}

func newMockSender() sender {
	mj := mockjob.NewMockJob()
	send := make(chan streaming.SearchEvent)
	mj.RunFunc.SetDefaultHook(func(_ context.Context, _ job.RuntimeClients, s streaming.Sender) (*search.Alert, error) {
		for event := range send {
			s.Send(event)
		}
		return nil, nil
	})
	return sender{Job: mj, sendC: send}
}

func newMockSenders(n int) senders {
	senders := make([]sender, 0, n)
	for range n {
		senders = append(senders, newMockSender())
	}
	return senders
}

func requireSoon(t *testing.T, c chan struct{}) {
	select {
	case <-c:
	case <-time.After(time.Second):
		t.Fatalf("expected an event to come within a second")
	}
}

func requireNotSoon(t *testing.T, c chan struct{}) {
	select {
	case <-c:
		t.Fatalf("unexpected event")
	case <-time.After(10 * time.Millisecond):
	}
}

func TestAndJob(t *testing.T) {
	t.Run("NewAndJob", func(t *testing.T) {
		t.Run("no children is simplified", func(t *testing.T) {
			require.Equal(t, NewNoopJob(), NewAndJob())
		})
		t.Run("one child is simplified", func(t *testing.T) {
			j := mockjob.NewMockJob()
			require.Equal(t, j, NewAndJob(j))
		})
	})

	t.Run("result returned from all subexpressions is streamed", func(t *testing.T) {
		for i := 2; i < 5; i++ {
			t.Run(fmt.Sprintf("%d subexpressions", i), func(t *testing.T) {
				senders := newMockSenders(i)
				j := NewAndJob(senders.Jobs()...)

				eventC := make(chan struct{}, 1)
				stream := streaming.StreamFunc(func(streaming.SearchEvent) { eventC <- struct{}{} })

				finished := make(chan struct{})
				go func() {
					_, err := j.Run(context.Background(), job.RuntimeClients{}, stream)
					require.NoError(t, err)
					close(finished)
				}()

				senders.SendAll()        // send the match from all jobs
				requireSoon(t, eventC)   // expect the AndJob to send an event
				senders.ExitAll()        // signal the jobs to exit
				requireSoon(t, finished) // expect our AndJob to exit soon
			})
		}
	})

	t.Run("result not returned from all subexpressions is not streamed", func(t *testing.T) {
		for i := 2; i < 5; i++ {
			t.Run(fmt.Sprintf("%d subexpressions", i), func(t *testing.T) {
				noSender := mockjob.NewMockJob()
				noSender.RunFunc.SetDefaultReturn(nil, nil)
				senders := newMockSenders(i)
				j := NewAndJob(append(senders.Jobs(), noSender)...)

				eventC := make(chan struct{}, 1)
				stream := streaming.StreamFunc(func(e streaming.SearchEvent) { eventC <- struct{}{} })

				finished := make(chan struct{})
				go func() {
					_, err := j.Run(context.Background(), job.RuntimeClients{}, stream)
					require.NoError(t, err)
					close(finished)
				}()

				senders.SendAll()         // send the match from all jobs but noSender
				requireNotSoon(t, eventC) // an event should NOT be streamed
				senders.ExitAll()         // signal the jobs to exit
				requireNotSoon(t, eventC) // an event should NOT be streamed after all jobs exit
				requireSoon(t, finished)  // expect our AndJob to exit soon
			})
		}
	})
}

func TestOrJob(t *testing.T) {
	t.Run("NoOrJob", func(t *testing.T) {
		t.Run("no children is simplified", func(t *testing.T) {
			require.Equal(t, NewNoopJob(), NewOrJob())
		})
		t.Run("one child is simplified", func(t *testing.T) {
			j := mockjob.NewMockJob()
			require.Equal(t, j, NewOrJob(j))
		})
	})

	t.Run("result returned from all subexpressions is streamed", func(t *testing.T) {
		for i := 2; i < 5; i++ {
			t.Run(fmt.Sprintf("%d subexpressions", i), func(t *testing.T) {
				senders := newMockSenders(i)
				j := NewOrJob(senders.Jobs()...)

				eventC := make(chan struct{}, 1)
				stream := streaming.StreamFunc(func(streaming.SearchEvent) { eventC <- struct{}{} })

				finished := make(chan struct{})
				go func() {
					_, err := j.Run(context.Background(), job.RuntimeClients{}, stream)
					require.NoError(t, err)
					close(finished)
				}()

				senders.SendAll()        // send the match from all jobs
				requireSoon(t, eventC)   // expect the OrJob to send an event
				senders.ExitAll()        // signal the jobs to exit
				requireSoon(t, finished) // expect our OrJob to exit soon
			})
		}
	})

	t.Run("result not streamed until all subexpression return the same result", func(t *testing.T) {
		noSender := mockjob.NewMockJob()
		noSender.RunFunc.SetDefaultReturn(nil, nil)

		for i := 2; i < 5; i++ {
			t.Run(fmt.Sprintf("%d subexpressions", i), func(t *testing.T) {
				noSender := mockjob.NewMockJob()
				noSender.RunFunc.SetDefaultReturn(nil, nil)
				senders := newMockSenders(i)
				j := NewOrJob(append(senders.Jobs(), noSender)...)

				eventC := make(chan struct{}, 1)
				stream := streaming.StreamFunc(func(e streaming.SearchEvent) { eventC <- struct{}{} })

				finished := make(chan struct{})
				go func() {
					_, err := j.Run(context.Background(), job.RuntimeClients{}, stream)
					require.NoError(t, err)
					close(finished)
				}()

				senders.SendAll()         // send the match from all jobs but noSender
				requireNotSoon(t, eventC) // an event should NOT be streamed
				senders.ExitAll()         // signal the jobs to exit
				requireSoon(t, eventC)    // an event SHOULD be streamed after all jobs exit
				requireSoon(t, finished)  // expect our AndJob to exit soon
			})
		}
	})

	t.Run("partial error still eventually sends results", func(t *testing.T) {
		errSender := mockjob.NewMockJob()
		errSender.RunFunc.SetDefaultReturn(nil, errors.New("test error"))
		senders := newMockSenders(2)
		j := NewOrJob(append(senders.Jobs(), errSender)...)

		stream := streaming.NewAggregatingStream()
		finished := make(chan struct{})
		go func() {
			_, err := j.Run(context.Background(), job.RuntimeClients{}, stream)
			require.Error(t, err)
			close(finished)
		}()

		senders.SendAll()
		senders.ExitAll()
		<-finished
		require.Len(t, stream.Results, 1)
	})
}
