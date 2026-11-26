package pythonapp

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

var hash = strings.Repeat("a", 64)

func project(extra map[string]string) fstest.MapFS {
	files := fstest.MapFS{
		"requirements.txt": {Data: []byte("flask==3.1.3 --hash=sha256:" + hash + "\n")},
		"Procfile":         {Data: []byte("web: gunicorn app:app --bind 0.0.0.0:$PORT\n")},
		"app.py":           {Data: []byte("print('hi')\n")},
	}
	for name, data := range extra {
		if data == "" {
			delete(files, name)
			continue
		}
		files[name] = &fstest.MapFile{Data: []byte(data)}
	}
	return files
}

func code(err error) string {
	var contract *ContractError
	if errors.As(err, &contract) {
		return contract.Code
	}
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestTheExampleMeetsTheContract(t *testing.T) {
	example := os.DirFS("../../dev/python-flask-example")
	if !IsPython(example) {
		t.Fatal("the example is not seen as Python")
	}
	if err := Validate(context.Background(), example); err != nil {
		t.Fatal(err)
	}
}

func TestContractNamesTheRuleThatFailed(t *testing.T) {
	for name, tc := range map[string]struct {
		files fs.FS
		want  string
	}{
		"valid":               {project(nil), ""},
		"no requirements":     {project(map[string]string{"requirements.txt": ""}), reasonRequirements},
		"unhashed":            {project(map[string]string{"requirements.txt": "flask==3.1.3\n"}), reasonRequirements},
		"unpinned":            {project(map[string]string{"requirements.txt": "flask --hash=sha256:" + hash}), reasonRequirements},
		"another index":       {project(map[string]string{"requirements.txt": "--extra-index-url https://evil.example\n"}), reasonRequirements},
		"empty requirements":  {project(map[string]string{"requirements.txt": "# nothing but the standard library\n"}), ""},
		"no Procfile":         {project(map[string]string{"Procfile": ""}), reasonProcfile},
		"no web process":      {project(map[string]string{"Procfile": "worker: python job.py\n"}), reasonProcfile},
		"two processes":       {project(map[string]string{"Procfile": "web: gunicorn app:app\nworker: python job.py\n"}), reasonProcfile},
		"blank web":           {project(map[string]string{"Procfile": "web:   \n"}), reasonProcfile},
		"comments are fine":   {project(map[string]string{"Procfile": "# the server\r\nweb: python app.py\r\n"}), ""},
		"uploaded venv":       {project(map[string]string{"venv/pyvenv.cfg": "home = /usr"}), reasonFiles},
		"uploaded packages":   {project(map[string]string{"site-packages/flask/__init__.py": "x"}), reasonFiles},
		"a Dockerfile":        {project(map[string]string{"Dockerfile": "FROM scratch"}), reasonFiles},
		"nested packages too": {project(map[string]string{"lib/site-packages/x.py": "x"}), reasonFiles},
		"an ordinary subdir":  {project(map[string]string{"templates/index.html": "<p>hi</p>"}), ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := code(Validate(context.Background(), tc.files)); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWebCommand(t *testing.T) {
	command, err := WebCommand([]byte("\n# comment\nweb:   uvicorn main:app --host 0.0.0.0 --port $PORT  \n"))
	if err != nil || command != "uvicorn main:app --host 0.0.0.0 --port $PORT" {
		t.Fatalf("%q %v", command, err)
	}
	if _, err := WebCommand([]byte("web: " + strings.Repeat("x", MaxCommand+1))); err == nil {
		t.Fatal("an overlong command was accepted")
	}
}
