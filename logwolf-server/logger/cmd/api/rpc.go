package main

import (
	"fmt"
	"log"
	"logwolf-toolbox/data"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RPCServer struct {
	models data.Models
}

func (r *RPCServer) LogInfo(p data.RPCLogPayload, resp *string) error {
	log.Printf("Logging info: %s", p.Name)

	err := r.models.Insert(data.LogEntry{
		ProjectID: p.ProjectID,
		Name:      p.Name,
		Data:      p.Data,
		Severity:  p.Severity,
		Tags:      p.Tags,
		Duration:  p.Duration,
	})
	if err != nil {
		log.Println("Error inserting into logs:", err)
		return err
	}

	*resp = fmt.Sprintf("Processed payload via RPC: %s", p.Name)
	return nil
}

func (r *RPCServer) GetLogs(p data.QueryParams, resp *[]data.LogEntry) error {
	log.Printf("Getting logs with params %+v...\n", p)

	result, err := r.models.AllLogs(p)
	if err != nil {
		log.Println("Error getting logs:", err)
		return err
	}

	for _, doc := range result {
		*resp = append(*resp, *doc)
	}

	log.Printf("Logs found via RPC: %d\n", len(*resp))
	return nil
}

func (r *RPCServer) DeleteLog(f data.RPCLogEntryFilter, resp *int64) error {
	log.Printf("Deleting log %+v...\n", f)

	result, err := r.models.DeleteLog(f.ID, f.ProjectID)
	if err != nil {
		log.Println("Error deleting document:", err)
		return err
	}

	*resp = result.DeletedCount
	log.Printf("Deleted: %d!", result.DeletedCount)

	return nil
}

func (r *RPCServer) GetRetention(args *data.RetentionArgs, reply *int) error {
	days, err := r.models.Settings.GetRetentionDays(args.ProjectID)
	if err != nil {
		return err
	}
	*reply = days
	return nil
}

func (r *RPCServer) UpdateRetention(args *data.RetentionArgs, reply *string) error {
	if err := r.models.Settings.SetRetentionDays(args.ProjectID, args.Days); err != nil {
		return err
	}
	*reply = "ok"
	return nil
}

func (r *RPCServer) GetMetrics(args *data.ProjectArgs, reply *data.Metrics) error {
	metrics, err := r.models.GetMetrics(args.ProjectID)
	if err != nil {
		return err
	}
	*reply = *metrics
	return nil
}

func (r *RPCServer) CreateProject(args *data.RPCCreateProjectArgs, reply *data.Project) error {
	log.Printf("Creating project: %s (%s)", args.Name, args.Slug)
	project, err := r.models.InsertProject(data.Project{Name: args.Name, Slug: args.Slug})
	if err != nil {
		log.Println("Error creating project:", err)
		return err
	}
	*reply = *project
	return nil
}

func (r *RPCServer) GetProject(args *data.RPCProjectIDArgs, reply *data.Project) error {
	log.Printf("Getting project: %s", args.ID)
	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return fmt.Errorf("GetProject: invalid ID: %w", err)
	}
	project, err := r.models.GetProject(id)
	if err != nil {
		log.Println("Error getting project:", err)
		return err
	}
	*reply = *project
	return nil
}

func (r *RPCServer) UpdateProject(args *data.RPCUpdateProjectArgs, reply *data.Project) error {
	log.Printf("Updating project: %s", args.ID)
	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return fmt.Errorf("UpdateProject: invalid ID: %w", err)
	}
	project, err := r.models.UpdateProject(id, args.Name, args.Slug)
	if err != nil {
		log.Println("Error updating project:", err)
		return err
	}
	*reply = *project
	return nil
}

func (r *RPCServer) DeleteProject(args *data.RPCProjectIDArgs, reply *string) error {
	log.Printf("Deleting project: %s", args.ID)
	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return fmt.Errorf("DeleteProject: invalid ID: %w", err)
	}
	if err := r.models.DeleteProject(id); err != nil {
		log.Println("Error deleting project:", err)
		return err
	}
	*reply = "ok"
	return nil
}

func (r *RPCServer) ListUserProjects(args *data.RPCUserProjectsArgs, reply *[]data.Project) error {
	log.Printf("Listing projects for user: %s", args.GithubLogin)
	projects, err := r.models.GetProjectsForUser(args.GithubLogin)
	if err != nil {
		log.Println("Error listing user projects:", err)
		return err
	}
	*reply = projects
	return nil
}

func (r *RPCServer) AddMember(args *data.RPCAddMemberArgs, reply *string) error {
	log.Printf("Adding member %s to project %s", args.GithubLogin, args.ProjectID)
	projectID, err := primitive.ObjectIDFromHex(args.ProjectID)
	if err != nil {
		return fmt.Errorf("AddMember: invalid project ID: %w", err)
	}
	_, err = r.models.InsertProjectMember(data.ProjectMember{
		ProjectID:   projectID,
		GithubLogin: args.GithubLogin,
		Role:        args.Role,
	})
	if err != nil {
		log.Println("Error adding member:", err)
		return err
	}
	*reply = "ok"
	return nil
}

func (r *RPCServer) RemoveMember(args *data.RPCMemberArgs, reply *string) error {
	log.Printf("Removing member %s from project %s", args.GithubLogin, args.ProjectID)
	projectID, err := primitive.ObjectIDFromHex(args.ProjectID)
	if err != nil {
		return fmt.Errorf("RemoveMember: invalid project ID: %w", err)
	}
	if err := r.models.RemoveProjectMember(projectID, args.GithubLogin); err != nil {
		log.Println("Error removing member:", err)
		return err
	}
	*reply = "ok"
	return nil
}

func (r *RPCServer) ListMembers(args *data.ProjectArgs, reply *[]data.ProjectMember) error {
	log.Printf("Listing members for project: %s", args.ProjectID)
	projectID, err := primitive.ObjectIDFromHex(args.ProjectID)
	if err != nil {
		return fmt.Errorf("ListMembers: invalid project ID: %w", err)
	}
	members, err := r.models.GetProjectMembers(projectID)
	if err != nil {
		log.Println("Error listing members:", err)
		return err
	}
	*reply = members
	return nil
}
