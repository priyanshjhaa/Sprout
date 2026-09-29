package deployment

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/priyanshjhaa/Sprout/backend/internal/authorization"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
)

type SQLRepository struct {
	pool          *pgxpool.Pool
	guardMu       sync.Mutex
	guard         *pgxpool.Conn
	guardRequired bool
}

func NewSQLRepository(pool *pgxpool.Pool) *SQLRepository { return &SQLRepository{pool: pool} }
func parseID(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return id, ErrInvalid
	}
	return id, nil
}
func scopeIDs(scope Scope) (pgtype.UUID, pgtype.UUID, error) {
	app, err := parseID(scope.ApplicationID)
	if err != nil {
		return app, pgtype.UUID{}, err
	}
	user, err := parseID(scope.UserID)
	return app, user, err
}
func permission(ctx context.Context, q *dbgen.Queries, scope Scope, edit bool) error {
	app, user, err := scopeIDs(scope)
	if err != nil {
		return err
	}
	row, err := q.GetApplicationAuthorization(ctx, dbgen.GetApplicationAuthorizationParams{WorkspaceSlug: scope.WorkspaceSlug, ApplicationID: app, UserID: user})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	access := authorization.ApplicationAccess{WorkspaceRole: authorization.WorkspaceRole(row.WorkspaceRole), Mode: authorization.AccessMode(row.AccessMode), IsCreator: row.IsCreator}
	if row.GrantRole.Valid {
		access.Grant = authorization.ApplicationRole(row.GrantRole.ApplicationRole)
	}
	if !access.CanView() {
		return ErrNotFound
	}
	if edit && !access.CanEdit() {
		return ErrForbidden
	}
	return nil
}
func (r *SQLRepository) Authorize(ctx context.Context, scope Scope, edit bool) error {
	return permission(ctx, dbgen.New(r.pool), scope, edit)
}
func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
func (r *SQLRepository) Create(ctx context.Context, scope Scope) (Job, error) {
	if err := r.CheckOwnership(ctx); err != nil {
		return Job{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer rollback(tx)
	q := dbgen.New(tx)
	if err = permission(ctx, q, scope, true); err != nil {
		return Job{}, err
	}
	app, user, err := scopeIDs(scope)
	if err != nil {
		return Job{}, err
	}
	row, err := q.CreateDeploymentSimulation(ctx, dbgen.CreateDeploymentSimulationParams{ApplicationID: app, UserID: user, WorkspaceSlug: scope.WorkspaceSlug})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return Job{}, ErrConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrConflict
	}
	if err != nil {
		return Job{}, err
	}
	if err = q.InitializeSimulationStages(ctx, row.ID); err != nil {
		return Job{}, err
	}
	job, err := mapJob(ctx, q, row)
	if err != nil {
		return Job{}, err
	}
	return job, tx.Commit(ctx)
}
func (r *SQLRepository) List(ctx context.Context, scope Scope) ([]Job, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	q := dbgen.New(tx)
	if err = permission(ctx, q, scope, false); err != nil {
		return nil, err
	}
	app, user, _ := scopeIDs(scope)
	rows, err := q.ListDeploymentSimulations(ctx, dbgen.ListDeploymentSimulationsParams{ApplicationID: app, UserID: user, WorkspaceSlug: scope.WorkspaceSlug})
	if err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(rows))
	for _, row := range rows {
		job, err := mapJob(ctx, q, row)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, tx.Commit(ctx)
}
func get(ctx context.Context, q *dbgen.Queries, scope Scope, id string) (dbgen.Deployment, error) {
	app, user, err := scopeIDs(scope)
	if err != nil {
		return dbgen.Deployment{}, err
	}
	jobID, err := parseID(id)
	if err != nil {
		return dbgen.Deployment{}, err
	}
	row, err := q.GetDeploymentSimulation(ctx, dbgen.GetDeploymentSimulationParams{ApplicationID: app, UserID: user, WorkspaceSlug: scope.WorkspaceSlug, DeploymentID: jobID})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, ErrNotFound
	}
	return row, err
}
func (r *SQLRepository) Get(ctx context.Context, scope Scope, id string) (Job, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Job{}, err
	}
	defer rollback(tx)
	q := dbgen.New(tx)
	row, err := get(ctx, q, scope, id)
	if err != nil {
		return Job{}, err
	}
	job, err := mapJob(ctx, q, row)
	if err != nil {
		return Job{}, err
	}
	return job, tx.Commit(ctx)
}
func (r *SQLRepository) Cancel(ctx context.Context, scope Scope, id string) (Job, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer rollback(tx)
	q := dbgen.New(tx)
	row, err := get(ctx, q, scope, id)
	if err != nil {
		return Job{}, err
	}
	if _, err = q.LockDeploymentSimulation(ctx, row.ID); err != nil {
		return Job{}, err
	}
	if err = permission(ctx, q, scope, true); err != nil {
		return Job{}, err
	}
	count, err := finish(ctx, q, row.ID, "cancelled", "job_cancelled")
	if err != nil {
		return Job{}, err
	}
	if count == 0 {
		return Job{}, ErrConflict
	}
	row, err = get(ctx, q, scope, id)
	if err != nil {
		return Job{}, err
	}
	job, err := mapJob(ctx, q, row)
	if err != nil {
		return Job{}, err
	}
	return job, tx.Commit(ctx)
}
func (r *SQLRepository) Start(ctx context.Context, scope Scope, id string) error {
	if err := r.CheckOwnership(ctx); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	q := dbgen.New(tx)
	jobID, err := parseID(id)
	if err != nil {
		return err
	}
	row, err := q.LockDeploymentSimulation(ctx, jobID)
	if err != nil {
		return err
	}
	if row.ApplicationID.String() != scope.ApplicationID || row.TriggeredBy.String() != scope.UserID {
		return ErrNotFound
	}
	if err = permission(ctx, q, scope, true); err != nil {
		return err
	}
	app, user, _ := scopeIDs(scope)
	application, err := q.GetApplicationForMember(ctx, dbgen.GetApplicationForMemberParams{ApplicationID: app, UserID: user, WorkspaceSlug: scope.WorkspaceSlug})
	if err != nil {
		return err
	}
	if application.Lifecycle != "active" {
		return ErrConflict
	}
	count, err := q.StartSimulation(ctx, jobID)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrConflict
	}
	return tx.Commit(ctx)
}
func (r *SQLRepository) Advance(ctx context.Context, id, stage, status string) error {
	if err := r.CheckOwnership(ctx); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	q := dbgen.New(tx)
	jobID, err := parseID(id)
	if err != nil {
		return err
	}
	row, err := q.LockDeploymentSimulation(ctx, jobID)
	if err != nil {
		return err
	}
	if row.Status != "building" {
		return ErrConflict
	}
	// Protect ordering in persistence, not just in the worker's loop.
	events, err := q.ListSimulationStages(ctx, jobID)
	if err != nil {
		return err
	}
	found := false
	for _, event := range events {
		if string(event.Stage) == stage {
			found = true
			break
		}
		if event.Status != "succeeded" {
			return ErrConflict
		}
	}
	if !found {
		return ErrInvalid
	}
	count, err := q.AdvanceSimulationStage(ctx, dbgen.AdvanceSimulationStageParams{DeploymentID: jobID, Stage: dbgen.DeploymentStage(stage), Status: dbgen.OperationStatus(status)})
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrConflict
	}
	return tx.Commit(ctx)
}
func (r *SQLRepository) Finish(ctx context.Context, id, status, code string) error {
	if err := r.CheckOwnership(ctx); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	q := dbgen.New(tx)
	jobID, err := parseID(id)
	if err != nil {
		return err
	}
	row, err := q.LockDeploymentSimulation(ctx, jobID)
	if err != nil {
		return err
	}
	if row.Status != "queued" && row.Status != "building" {
		return nil
	} // cancellation wins over late completion
	if status == "succeeded" {
		events, err := q.ListSimulationStages(ctx, jobID)
		if err != nil {
			return err
		}
		if len(events) != len(stages) {
			return ErrConflict
		}
		for _, event := range events {
			if event.Status != "succeeded" {
				return ErrConflict
			}
		}
	}
	if _, err = finish(ctx, q, jobID, status, code); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func finish(ctx context.Context, q *dbgen.Queries, id pgtype.UUID, status, code string) (int64, error) {
	if status != "succeeded" && status != "failed" && status != "cancelled" {
		return 0, ErrInvalid
	}
	failure := pgtype.Text{String: code, Valid: code != ""}
	count, err := q.FinishSimulation(ctx, dbgen.FinishSimulationParams{ID: id, Status: dbgen.DeploymentStatus(status), FailureCode: failure})
	if err != nil || count == 0 {
		return count, err
	}
	if status != "succeeded" {
		err = q.FinishSimulationStages(ctx, dbgen.FinishSimulationStagesParams{DeploymentID: id, FailureCode: failure})
	}
	return count, err
}
func mapJob(ctx context.Context, q *dbgen.Queries, row dbgen.Deployment) (Job, error) {
	job := Job{ID: row.ID.String(), ApplicationID: row.ApplicationID.String(), Simulated: row.Simulated, Status: string(row.Status), CreatedAt: row.CreatedAt.Time, Stages: []Stage{}}
	if row.FailureCode.Valid {
		job.FailureCode = &row.FailureCode.String
	}
	if row.StartedAt.Valid {
		job.StartedAt = &row.StartedAt.Time
	}
	if row.FinishedAt.Valid {
		job.FinishedAt = &row.FinishedAt.Time
	}
	events, err := q.ListSimulationStages(ctx, row.ID)
	if err != nil {
		return Job{}, err
	}
	for _, event := range events {
		stage := Stage{Name: string(event.Stage), Status: string(event.Status)}
		if event.FailureCode.Valid {
			stage.FailureCode = &event.FailureCode.String
		}
		job.Stages = append(job.Stages, stage)
	}
	return job, nil
}

// AcquireProcessLock deliberately supports one simulation-owning API process.
// A dedicated DB connection holds a session lock; startup fails if already held.
// Recovery happens only after ownership is acquired, never across live replicas.
func (r *SQLRepository) AcquireProcessLock(ctx context.Context) (func(), error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	var acquired bool
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(hashtext(current_schema()), 1936749167)").Scan(&acquired); err != nil || !acquired {
		conn.Release()
		if err != nil {
			return nil, err
		}
		return nil, ErrUnavailable
	}
	released := false
	release := func() {
		r.guardMu.Lock()
		defer r.guardMu.Unlock()
		if released {
			return
		}
		released = true
		if r.guard == conn {
			r.guard = nil
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		// Close instead of returning a possibly still-locked connection to the pool.
		_ = conn.Conn().Close(cleanup)
		conn.Release()
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		release()
		return nil, err
	}
	q := dbgen.New(tx)
	ids, err := q.RecoverInterruptedSimulations(ctx)
	if err == nil {
		for _, id := range ids {
			err = q.FinishSimulationStages(ctx, dbgen.FinishSimulationStagesParams{DeploymentID: id, FailureCode: pgtype.Text{String: "process_interrupted", Valid: true}})
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		rollback(tx)
		release()
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		release()
		return nil, err
	}
	r.guardMu.Lock()
	r.guard = conn
	r.guardRequired = true
	r.guardMu.Unlock()
	return release, nil
}

// A dead ownership connection must not be silently replaced by a pooled one.
// This is a single-process development worker, not a distributed lease system.
func (r *SQLRepository) CheckOwnership(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	r.guardMu.Lock()
	defer r.guardMu.Unlock()
	if r.guard == nil {
		if r.guardRequired {
			return ErrUnavailable
		}
		return nil
	} // Repository-only tests do not run a process worker.
	if r.guard.Conn() == nil || r.guard.Conn().IsClosed() {
		return ErrUnavailable
	}
	if err := r.guard.Ping(ctx); err != nil {
		return ErrUnavailable
	}
	return nil
}
