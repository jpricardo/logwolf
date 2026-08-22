package main

import (
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"testing"

	"logwolf-toolbox/data"
)

// fakeLogger stands in for the Logger service over a real net/rpc connection.
//
// The broker's project handlers dial the logger themselves — the address comes
// from LOGGER_RPC_ADDR, not from Config — so the only way to exercise the
// membership checks is to put a server on the other end of that dial. Serving
// the same method set over gob keeps the wire contract (including net/rpc's
// flattening of errors into strings, which the broker matches on) intact.
//
// All exported methods must be RPC-shaped; helpers for the tests are unexported
// so net/rpc ignores them.
type fakeLogger struct {
	mu sync.Mutex

	projects  map[string]data.Project         // project id hex -> project
	members   map[string][]data.ProjectMember // project id hex -> members
	logs      map[string][]data.LogEntry      // project id hex -> logs
	retention map[string]int                  // project id hex -> days
	metrics   map[string]data.Metrics         // project id hex -> metrics

	// Recorded calls, for asserting what the broker forwarded.
	getLogsParams   []data.QueryParams
	retentionArgs   []data.RetentionArgs
	metricsArgs     []data.ProjectArgs
	createdProjects []data.RPCCreateProjectArgs
	updatedProjects []data.RPCUpdateProjectArgs
	deletedProjects []string
	addedMembers    []data.RPCAddMemberArgs
	removedMembers  []data.RPCRemoveMemberArgs

	// Failure injection.
	duplicateSlug  string // CreateProject returns an E11000 error for this slug
	failAddMember  bool   // AddMember always fails (exercises the create-project rollback)
	lastOwnerLogin string // RemoveMember refuses to remove this login
}

// errNoDocuments mirrors the driver error the logger passes back when a lookup
// misses. The broker only sees the message, so the wording is the contract.
var errNoDocuments = errors.New("mongo: no documents in result set")

func newFakeLogger() *fakeLogger {
	return &fakeLogger{
		projects:  map[string]data.Project{},
		members:   map[string][]data.ProjectMember{},
		logs:      map[string][]data.LogEntry{},
		retention: map[string]int{},
		metrics:   map[string]data.Metrics{},
	}
}

// --- RPC surface ---

func (f *fakeLogger) GetProject(args *data.RPCProjectIDArgs, reply *data.Project) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	p, ok := f.projects[args.ID]
	if !ok {
		return errNoDocuments
	}
	*reply = p
	return nil
}

func (f *fakeLogger) CreateProject(args *data.RPCCreateProjectArgs, reply *data.Project) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.createdProjects = append(f.createdProjects, *args)
	if args.Slug == f.duplicateSlug {
		return fmt.Errorf("E11000 duplicate key error collection: logs.projects index: slug_1")
	}

	id := nextProjectID()
	p := data.Project{ID: mustObjectID(id), Name: args.Name, Slug: args.Slug}
	f.projects[id] = p
	*reply = p
	return nil
}

func (f *fakeLogger) UpdateProject(args *data.RPCUpdateProjectArgs, reply *data.Project) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.updatedProjects = append(f.updatedProjects, *args)
	p, ok := f.projects[args.ID]
	if !ok {
		return errNoDocuments
	}
	p.Name, p.Slug = args.Name, args.Slug
	f.projects[args.ID] = p
	*reply = p
	return nil
}

func (f *fakeLogger) DeleteProject(args *data.RPCProjectIDArgs, reply *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deletedProjects = append(f.deletedProjects, args.ID)
	delete(f.projects, args.ID)
	delete(f.members, args.ID)
	delete(f.logs, args.ID)
	delete(f.retention, args.ID)
	*reply = "ok"
	return nil
}

func (f *fakeLogger) ListUserProjects(args *data.RPCUserProjectsArgs, reply *[]data.UserProject) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []data.UserProject
	for id, members := range f.members {
		for _, m := range members {
			if m.GithubLogin != args.GithubLogin {
				continue
			}
			if p, ok := f.projects[id]; ok {
				out = append(out, data.UserProject{Project: p, Role: m.Role})
			}
		}
	}
	*reply = out
	return nil
}

func (f *fakeLogger) ListMembers(args *data.ProjectArgs, reply *[]data.ProjectMember) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	*reply = append([]data.ProjectMember(nil), f.members[args.ProjectID]...)
	return nil
}

func (f *fakeLogger) CheckMembership(args *data.RPCCheckMembershipArgs, reply *bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, m := range f.members[args.ProjectID] {
		if m.GithubLogin == args.GithubLogin {
			*reply = true
			return nil
		}
	}
	*reply = false
	return nil
}

func (f *fakeLogger) AddMember(args *data.RPCAddMemberArgs, reply *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.addedMembers = append(f.addedMembers, *args)
	if f.failAddMember {
		return fmt.Errorf("AddMember: injected failure")
	}
	f.members[args.ProjectID] = append(f.members[args.ProjectID], data.ProjectMember{
		ProjectID:   mustObjectID(args.ProjectID),
		GithubLogin: args.GithubLogin,
		Role:        args.Role,
	})
	*reply = "ok"
	return nil
}

func (f *fakeLogger) RemoveMember(args *data.RPCRemoveMemberArgs, reply *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.removedMembers = append(f.removedMembers, *args)
	if args.GithubLogin == f.lastOwnerLogin {
		// data.ErrLastOwner's message, as the broker sees it over the wire.
		return fmt.Errorf("cannot remove the last owner of a project")
	}

	kept := f.members[args.ProjectID][:0]
	for _, m := range f.members[args.ProjectID] {
		if m.GithubLogin != args.GithubLogin {
			kept = append(kept, m)
		}
	}
	f.members[args.ProjectID] = kept
	*reply = "ok"
	return nil
}

func (f *fakeLogger) GetLogs(p data.QueryParams, reply *[]data.LogEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.getLogsParams = append(f.getLogsParams, p)
	*reply = append([]data.LogEntry(nil), f.logs[p.ProjectID]...)
	return nil
}

func (f *fakeLogger) GetLog(filter data.RPCLogEntryFilter, reply *data.LogEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, e := range f.logs[filter.ProjectID] {
		if e.ID == filter.ID {
			*reply = e
			return nil
		}
	}
	return errNoDocuments
}

func (f *fakeLogger) DeleteLog(filter data.RPCLogEntryFilter, reply *int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var deleted int64
	kept := make([]data.LogEntry, 0, len(f.logs[filter.ProjectID]))
	for _, e := range f.logs[filter.ProjectID] {
		if e.ID == filter.ID {
			deleted++
			continue
		}
		kept = append(kept, e)
	}
	f.logs[filter.ProjectID] = kept
	*reply = deleted
	return nil
}

func (f *fakeLogger) GetRetention(args *data.RetentionArgs, reply *int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.retentionArgs = append(f.retentionArgs, *args)
	days, ok := f.retention[args.ProjectID]
	if !ok {
		days = 90
	}
	*reply = days
	return nil
}

func (f *fakeLogger) UpdateRetention(args *data.RetentionArgs, reply *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.retentionArgs = append(f.retentionArgs, *args)
	f.retention[args.ProjectID] = args.Days
	*reply = "ok"
	return nil
}

func (f *fakeLogger) GetMetrics(args *data.ProjectArgs, reply *data.Metrics) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.metricsArgs = append(f.metricsArgs, *args)
	*reply = f.metrics[args.ProjectID]
	return nil
}

// --- test-side helpers (unexported, so net/rpc ignores them) ---

func (f *fakeLogger) addProject(id, name, slug string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projects[id] = data.Project{ID: mustObjectID(id), Name: name, Slug: slug}
}

func (f *fakeLogger) addMember(projectID, login, role string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.members[projectID] = append(f.members[projectID], data.ProjectMember{
		ProjectID:   mustObjectID(projectID),
		GithubLogin: login,
		Role:        role,
	})
}

func (f *fakeLogger) addLog(projectID, logID, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs[projectID] = append(f.logs[projectID], data.LogEntry{
		ID:        logID,
		ProjectID: projectID,
		Name:      name,
		Data:      "{}",
		Severity:  "info",
		Tags:      []string{},
	})
}

func (f *fakeLogger) snapshot(read func(*fakeLogger)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	read(f)
}

// serveFakeLogger publishes f on a loopback listener under the name the broker
// calls ("RPCServer") and points LOGGER_RPC_ADDR at it for the duration of t.
func serveFakeLogger(t *testing.T, f *fakeLogger) {
	t.Helper()

	srv := rpc.NewServer()
	if err := srv.RegisterName("RPCServer", f); err != nil {
		t.Fatalf("register fake logger: %v", err)
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { l.Close() })

	// Accept returns as soon as the listener closes, which Cleanup handles.
	go srv.Accept(l)

	t.Setenv("LOGGER_RPC_ADDR", l.Addr().String())
}
