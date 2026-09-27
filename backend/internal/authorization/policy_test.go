package authorization

import "testing"

func TestWorkspacePermissions(t *testing.T) {
	for _, test := range []struct {
		role         WorkspaceRole
		create, team bool
		delete       bool
	}{
		{Owner, true, true, true},
		{Editor, true, false, false},
		{Viewer, false, false, false},
		{"", false, false, false},
	} {
		if CanCreateApplication(test.role) != test.create ||
			CanManageMembers(test.role) != test.team ||
			CanDeleteApplication(test.role) != test.delete {
			t.Errorf("unexpected workspace permissions for %q", test.role)
		}
	}
}

func TestApplicationPermissions(t *testing.T) {
	for _, test := range []struct {
		name               string
		access             ApplicationAccess
		view, edit, manage bool
	}{
		{"owner on restricted", ApplicationAccess{WorkspaceRole: Owner, Mode: Restricted}, true, true, true},
		{"creator on restricted", ApplicationAccess{WorkspaceRole: Editor, Mode: Restricted, IsCreator: true}, true, true, true},
		{"editor on workspace-wide", ApplicationAccess{WorkspaceRole: Editor, Mode: WorkspaceWide}, true, true, false},
		{"viewer on workspace-wide", ApplicationAccess{WorkspaceRole: Viewer, Mode: WorkspaceWide}, true, false, false},
		{"viewer with editor grant", ApplicationAccess{WorkspaceRole: Viewer, Grant: ApplicationEditor, Mode: WorkspaceWide}, true, true, false},
		{"editor without restricted grant", ApplicationAccess{WorkspaceRole: Editor, Mode: Restricted}, false, false, false},
		{"viewer with restricted grant", ApplicationAccess{WorkspaceRole: Viewer, Grant: ApplicationViewer, Mode: Restricted}, true, false, false},
		{"editor with restricted grant", ApplicationAccess{WorkspaceRole: Editor, Grant: ApplicationEditor, Mode: Restricted}, true, true, false},
		{"former member with grant", ApplicationAccess{Grant: ApplicationEditor, Mode: Restricted, IsCreator: true}, false, false, false},
		{"unknown mode fails closed", ApplicationAccess{WorkspaceRole: Owner}, false, false, false},
	} {
		if test.access.CanView() != test.view || test.access.CanEdit() != test.edit ||
			test.access.CanManageAccess() != test.manage {
			t.Errorf("unexpected application permissions for %s", test.name)
		}
	}
}
