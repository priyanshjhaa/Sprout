package authorization

type WorkspaceRole string

const (
	Owner  WorkspaceRole = "owner"
	Editor WorkspaceRole = "editor"
	Viewer WorkspaceRole = "viewer"
)

type ApplicationRole string

const (
	ApplicationEditor ApplicationRole = "editor"
	ApplicationViewer ApplicationRole = "viewer"
)

type AccessMode string

const (
	WorkspaceWide AccessMode = "workspace"
	Restricted    AccessMode = "restricted"
)

type ApplicationAccess struct {
	WorkspaceRole WorkspaceRole
	Grant         ApplicationRole
	Mode          AccessMode
	IsCreator     bool
}

func (access ApplicationAccess) CanView() bool {
	if !validWorkspaceRole(access.WorkspaceRole) || !validMode(access.Mode) {
		return false
	}
	if access.WorkspaceRole == Owner || access.IsCreator {
		return true
	}
	if access.Grant == ApplicationEditor || access.Grant == ApplicationViewer {
		return true
	}
	return access.Mode == WorkspaceWide
}

func (access ApplicationAccess) CanEdit() bool {
	if !access.CanView() {
		return false
	}
	if access.WorkspaceRole == Owner || access.IsCreator || access.Grant == ApplicationEditor {
		return true
	}
	return access.Mode == WorkspaceWide && access.WorkspaceRole == Editor
}

func (access ApplicationAccess) CanManageAccess() bool {
	return access.CanView() && (access.WorkspaceRole == Owner || access.IsCreator)
}

func CanCreateApplication(role WorkspaceRole) bool {
	return role == Owner || role == Editor
}

func CanManageMembers(role WorkspaceRole) bool {
	return role == Owner
}

func CanDeleteApplication(role WorkspaceRole) bool {
	return role == Owner
}

func validWorkspaceRole(role WorkspaceRole) bool {
	return role == Owner || role == Editor || role == Viewer
}

func validMode(mode AccessMode) bool {
	return mode == WorkspaceWide || mode == Restricted
}
