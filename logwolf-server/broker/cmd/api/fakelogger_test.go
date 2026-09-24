package main

import (
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"sync/atomic"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

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
	keys      map[string]data.APIKey          // key id hex -> key
	plaintext map[string]string               // plaintext key -> key id hex

	// Recorded calls, for asserting what the broker forwarded.
	getLogsParams   []data.QueryParams
	retentionArgs   []data.RetentionArgs
	metricsArgs     []data.ProjectArgs
	createdProjects []data.RPCCreateProjectArgs
	updatedProjects []data.RPCUpdateProjectArgs
	deletedProjects []string
	addedMembers    []data.RPCAddMemberArgs
	removedMembers  []data.RPCRemoveMemberArgs
	roleChanges     []data.RPCUpdateMemberRoleArgs
	revokedKeys     []data.RPCRevokeAPIKeyArgs

	// Failure injection.
	failCreateProject bool   // CreateProject fails, as its transaction would, and creates nothing
	lastOwnerLogin    string // RemoveMember and UpdateMemberRole refuse to remove or demote this login

	// openConns counts the broker's connections the fake has not yet seen
	// closed. It goes back to zero only if every handler closed its client.
	openConns atomic.Int64
}

// errNoDocuments mirrors the driver error the logger passes back when a lookup
// misses. The broker only sees the message, so the wording is the contract.
var errNoDocuments = errors.New("mongo: no documents in result set")

// checkObjectID refuses a malformed id the way the logger's RPC methods do,
// before they touch the database.
func checkObjectID(op, id string) error {
	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		return fmt.Errorf("%s: invalid project ID: %w", op, err)
	}
	return nil
}

// errDuplicateKey mirrors the driver's unique-index violation, which the broker
// recognizes by its E11000 code.
func errDuplicateKey(index string) error {
	return fmt.Errorf("E11000 duplicate key error collection: logs index: %s", index)
}

func newFakeLogger() *fakeLogger {
	return &fakeLogger{
		projects:  map[string]data.Project{},
		members:   map[string][]data.ProjectMember{},
		logs:      map[string][]data.LogEntry{},
		retention: map[string]int{},
		metrics:   map[string]data.Metrics{},
		keys:      map[string]data.APIKey{},
		plaintext: map[string]string{},
	}
}

// --- RPC surface ---

func (f *fakeLogger) GetProject(args *data.RPCProjectIDArgs, reply *data.Project) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := checkObjectID("GetProject", args.ID); err != nil {
		return err
	}
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
	if f.failCreateProject {
		return fmt.Errorf("CreateProjectWithOwner owner: injected failure")
	}

	// Like the logger, the project and its owner come into being together.
	id := nextProjectID()
	p := data.Project{ID: mustObjectID(id), Name: args.Name, Slug: args.Slug}
	f.projects[id] = p
	f.members[id] = []data.ProjectMember{{ProjectID: p.ID, GithubLogin: args.Owner, Role: data.RoleOwner}}
	*reply = p
	return nil
}

func (f *fakeLogger) UpdateProject(args *data.RPCUpdateProjectArgs, reply *data.Project) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.updatedProjects = append(f.updatedProjects, *args)
	if err := checkObjectID("UpdateProject", args.ID); err != nil {
		return err
	}
	p, ok := f.projects[args.ID]
	if !ok {
		return errNoDocuments
	}
	p.Name = args.Name
	f.projects[args.ID] = p
	*reply = p
	return nil
}

func (f *fakeLogger) DeleteProject(args *data.RPCProjectIDArgs, reply *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := checkObjectID("DeleteProject", args.ID); err != nil {
		return err
	}
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

	if err := checkObjectID("ListMembers", args.ProjectID); err != nil {
		return err
	}
	*reply = append([]data.ProjectMember(nil), f.members[args.ProjectID]...)
	return nil
}

func (f *fakeLogger) CheckMembership(args *data.RPCCheckMembershipArgs, reply *bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := checkObjectID("CheckMembership", args.ProjectID); err != nil {
		return err
	}
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
	if err := checkObjectID("AddMember", args.ProjectID); err != nil {
		return err
	}
	for _, m := range f.members[args.ProjectID] {
		if m.GithubLogin == args.GithubLogin {
			return fmt.Errorf("InsertProjectMember: %w", errDuplicateKey("unique_project_member"))
		}
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
	if len(kept) == len(f.members[args.ProjectID]) {
		return fmt.Errorf("RemoveProjectMember: %w", errNoDocuments)
	}
	f.members[args.ProjectID] = kept
	*reply = "ok"
	return nil
}

func (f *fakeLogger) UpdateMemberRole(args *data.RPCUpdateMemberRoleArgs, reply *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.roleChanges = append(f.roleChanges, *args)
	if args.GithubLogin == f.lastOwnerLogin && args.Role != data.RoleOwner {
		return fmt.Errorf("UpdateProjectMemberRole: cannot remove the last owner of a project")
	}

	for i, m := range f.members[args.ProjectID] {
		if m.GithubLogin == args.GithubLogin {
			f.members[args.ProjectID][i].Role = args.Role
			*reply = "ok"
			return nil
		}
	}
	return fmt.Errorf("UpdateProjectMemberRole: %w", errNoDocuments)
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
	if !data.ValidRetentionDays[args.Days] {
		return fmt.Errorf("SetRetentionDays: %d is not a valid retention value", args.Days)
	}
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

func (f *fakeLogger) ValidateAPIKey(args *data.RPCValidateAPIKeyArgs, reply *data.RPCValidateAPIKeyReply) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	id, ok := f.plaintext[args.Plaintext]
	if !ok || !f.keys[id].Active {
		return nil
	}
	reply.Valid = true
	reply.Key = f.keys[id]
	return nil
}

func (f *fakeLogger) ListAPIKeys(args *data.ProjectArgs, reply *[]data.APIKey) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []data.APIKey
	for _, k := range f.keys {
		if k.ProjectID.Hex() == args.ProjectID {
			out = append(out, k)
		}
	}
	*reply = out
	return nil
}

func (f *fakeLogger) CreateAPIKey(args *data.RPCCreateAPIKeyArgs, reply *data.RPCCreateAPIKeyReply) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	scopes, err := data.NormalizeScopes(args.Scopes)
	if err != nil {
		return fmt.Errorf("CreateAPIKey: %w", err)
	}
	reply.Plaintext, reply.Key = f.storeKey(args.ProjectID, scopes)
	return nil
}

func (f *fakeLogger) GetAPIKey(args *data.RPCAPIKeyIDArgs, reply *data.APIKey) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, err := primitive.ObjectIDFromHex(args.ID); err != nil {
		return err
	}
	k, ok := f.keys[args.ID]
	if !ok {
		return data.ErrKeyNotFound
	}
	*reply = k
	return nil
}

func (f *fakeLogger) RevokeAPIKey(args *data.RPCRevokeAPIKeyArgs, reply *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.revokedKeys = append(f.revokedKeys, *args)
	k, ok := f.keys[args.ID]
	if !ok || k.ProjectID.Hex() != args.ProjectID {
		return data.ErrKeyNotFound
	}
	k.Active = false
	f.keys[args.ID] = k
	*reply = "ok"
	return nil
}

// --- test-side helpers (unexported, so net/rpc ignores them) ---

var keySeq atomic.Int64

// storeKey mints a key the way the logger would, minus the hashing, and keeps
// its plaintext so ValidateAPIKey can find it. The caller holds f.mu.
func (f *fakeLogger) storeKey(projectID string, scopes []string) (string, data.APIKey) {
	n := keySeq.Add(1)
	id := fmt.Sprintf("eeeeeeeeeeeeeeeeeeee%04d", n)
	plaintext := fmt.Sprintf("lw_fakelogger%033d", n)
	k := data.APIKey{
		ID:        mustObjectID(id),
		ProjectID: mustObjectID(projectID),
		Prefix:    plaintext[:10],
		Scopes:    scopes,
		Active:    true,
	}
	f.keys[id] = k
	f.plaintext[plaintext] = id
	return plaintext, k
}

func (f *fakeLogger) addKey(projectID string, scopes ...string) (plaintext string, key data.APIKey) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.storeKey(projectID, scopes)
}

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
		ProjectID: mustObjectID(projectID),
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
	go srv.Accept(countingListener{Listener: l, open: &f.openConns})

	t.Setenv("LOGGER_RPC_ADDR", l.Addr().String())
}

// countingListener tracks how many accepted connections are still open. The
// RPC server closes its end once the broker closes the client, so a connection
// the broker leaks stays counted.
type countingListener struct {
	net.Listener
	open *atomic.Int64
}

func (l countingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	l.open.Add(1)
	return &countedConn{Conn: c, open: l.open}, nil
}

type countedConn struct {
	net.Conn
	open   *atomic.Int64
	closed sync.Once
}

func (c *countedConn) Close() error {
	c.closed.Do(func() { c.open.Add(-1) })
	return c.Conn.Close()
}
