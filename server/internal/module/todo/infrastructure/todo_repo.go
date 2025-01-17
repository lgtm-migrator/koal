package todoinfra

import (
	"context"
	"errors"

	tododomain "github.com/glyphack/koal/internal/module/todo/domain/todo"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// Database access not found errors have to be converted to this error
var NotFoundErr = errors.New("Entry does not exist")

type Status uint

const (
	Both Status = iota
	Done
	NotDone
)

type TodoItemQuery struct {
	UUId       uuid.UUID
	Title      string
	IsDone     Status
	ProjectIds []uuid.UUID
	OwnerId    string
}

type TodoRepository interface {
	GetItemById(ctx context.Context, Id string) (*tododomain.TodoItem, error)
	AllUndoneItems(ctx context.Context, ownerId string) ([]*tododomain.TodoItem, error)
	GetItems(ctx context.Context, itemQuery TodoItemQuery) (*[]tododomain.TodoItem, error)
	CreateItem(ctx context.Context, newItem *tododomain.TodoItem) error
	UpdateItem(ctx context.Context, Id string, updatedItem *tododomain.TodoItem) error
	DeleteItem(ctx context.Context, ID string) error
	GetAllMemberProjects(ctx context.Context, OwnerId string) ([]*tododomain.Project, error)
	CreateProject(ctx context.Context, project *tododomain.Project) error
	GetProject(ctx context.Context, ID string) (*tododomain.ProjectInfo, error)
	DeleteProject(ctx context.Context, ID string) error
	UpdateProjectById(ctx context.Context, ID string, name string) error
}
