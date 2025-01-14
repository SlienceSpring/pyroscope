package agent

import (
	"os"

	"github.com/sirupsen/logrus"

	"github.com/pyroscope-io/pyroscope/pkg/agent/csock"
	"github.com/pyroscope-io/pyroscope/pkg/agent/upstream"
	"github.com/pyroscope-io/pyroscope/pkg/agent/upstream/remote"
	"github.com/pyroscope-io/pyroscope/pkg/config"
	"github.com/pyroscope-io/pyroscope/pkg/util/id"
)

type Agent struct {
	cfg            *config.Config
	cs             *csock.CSock
	activeProfiles map[int]*ProfileSession
	id             id.ID
	u              upstream.Upstream
}

func New(cfg *config.Config) *Agent {
	// TODO: handle this error properly
	r, _ := remote.New(remote.RemoteConfig{
		UpstreamThreads:        cfg.Agent.UpstreamThreads,
		UpstreamAddress:        cfg.Agent.ServerAddress,
		UpstreamRequestTimeout: cfg.Agent.UpstreamRequestTimeout,
	})
	return &Agent{
		cfg:            cfg,
		activeProfiles: make(map[int]*ProfileSession),
		u:              r,
	}
}

func (a *Agent) Start() {
	sockPath := a.cfg.Agent.UNIXSocketPath
	cs, err := csock.NewUnixCSock(sockPath, a.controlSocketHandler)
	if err != nil {
		logrus.Fatal(err)
	}
	a.cs = cs
	defer os.Remove(sockPath)

	go SelfProfile(a.cfg, a.u, "pyroscope.agent.cpu{}")
	logrus.WithField("addr", cs.CanonicalAddr()).Info("Starting control socket")
	cs.Start()
}

func (a *Agent) Stop() {
	a.cs.Stop()
}

func (a *Agent) controlSocketHandler(req *csock.Request) *csock.Response {
	switch req.Command {
	case "start":
		profileID := int(a.id.Next())
		// TODO: pass withSubprocesses from somewhere
		// TODO: pass appName from somewhere
		// TODO: add sample rate
		s := NewSession(a.u, "testapp.cpu", "gospy", 100, 0, false)
		a.activeProfiles[profileID] = s
		s.Start()
		return &csock.Response{ProfileID: profileID}
	case "stop":
		// TODO: "testapp.cpu{}" should come from the client
		profileID := req.ProfileID
		if s, ok := a.activeProfiles[profileID]; ok {
			s.Stop()
			delete(a.activeProfiles, profileID)
		}
		return &csock.Response{}
	default:
		return &csock.Response{}
	}
}
