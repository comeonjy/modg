package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"regexp"
	"strings"
)

type PackageTree struct {
	Children    []*PackageTree
	FilePath    string
	PackageName string
	Imports     []string
}

func main() {
	fileList := []string{
		//"doc/swagger/main.go",
		//"engine/agent-service/Makefile",
		"engine/agent-service/app/api/agent.go",
		//"engine/agent-service/docs/.gitkeep",
		//"engine/agent-service/docs/docs.go",
		//"engine/agent-service/docs/swagger.js",
	}
	projectName := "gaea"
	dir := "/Users/jiangyang/tr/gaea"
	packageTreeRoot := &PackageTree{PackageName: projectName, FilePath: dir, Children: make([]*PackageTree, 0)}
	if err := packageTreeRoot.Add(dir); err != nil {
		fmt.Println(err)
		return
	}
	//packageTreeRoot.Print()

	// 找出fileList影响的服务
	for _, v := range fileList {
		if strings.HasSuffix(v, "_test.go") || !strings.HasSuffix(v, ".go") {
			continue
		}
		fmt.Println(v, packageTreeRoot.Find(fmt.Sprintf("%s/%s", projectName, path.Dir(v))))
	}

}

func (t *PackageTree) Find(packageName string) []string {
	fmt.Println(packageName)
	affectedPackages := make([]string, 0)
	for _, v := range t.Imports {
		if v == packageName {
			if t.PackageName != "main" {
				affectedPackage := t.Find(strings.Split(t.FilePath, "gaea")[1])
				affectedPackages = append(affectedPackages, affectedPackage...)
			}
			affectedPackages = append(affectedPackages, t.FilePath)
		}
	}
	for _, v := range t.Children {
		affectedPackage := v.Find(packageName)
		affectedPackages = append(affectedPackages, affectedPackage...)
	}
	return affectedPackages
}

func (t *PackageTree) Print() {
	fmt.Println(t.FilePath, t.PackageName, t.Imports)
	for _, v := range t.Children {
		v.Print()
	}

}

func (t *PackageTree) Add(dir string) error {
	readDir, err := os.ReadDir(dir)
	if err != nil {
		return errors.New(fmt.Sprintf("read dir %s error: %s", dir, err.Error()))
	}
	fileList := make([]string, 0)
	for _, v := range readDir {
		if strings.HasPrefix(v.Name(), ".") {
			continue
		}
		if strings.HasSuffix(v.Name(), "_test.go") {
			continue
		}
		if v.IsDir() {
			if err := t.Add(fmt.Sprintf("%s/%s", dir, v.Name())); err != nil {
				return err
			}
		}
		if strings.HasSuffix(v.Name(), ".go") {
			fileList = append(fileList, fmt.Sprintf("%s/%s", dir, v.Name()))
		}
	}
	if len(fileList) == 0 {
		return nil
	}

	packageName, imports, err := ParseFileList(fileList)
	if err != nil {
		return err
	}
	t.Children = append(t.Children, &PackageTree{PackageName: packageName, Imports: imports, FilePath: dir, Children: make([]*PackageTree, 0)})

	return nil
}

var goBuildIgnoreRegex = regexp.MustCompile(`^//(go:build\s+.*|\s*\+build\s+.*)\s*$`)

func ParseFileList(filenames []string) (string, []string, error) {
	importsMap := make(map[string]struct{})
	imports := make([]string, 0)
	var packageName string
	for _, filename := range filenames {
		fst := token.NewFileSet()
		file, err := parser.ParseFile(fst, filename, nil, parser.ParseComments)
		if err != nil {
			return "", nil, errors.New(fmt.Sprintf("parse file error: %s", err.Error()))
		}
		if IsGoBuildIgnore(file.Comments) {
			continue
		}
		if len(packageName) > 0 && packageName != file.Name.Name {
			return "", nil, errors.New(fmt.Sprintf("package name not match: %s %s %s", filename, packageName, file.Name.Name))
		}
		packageName = file.Name.Name
		for _, v := range file.Imports {
			if _, ok := importsMap[v.Path.Value]; !ok {
				imports = append(imports, v.Path.Value)
				importsMap[v.Path.Value] = struct{}{}
			}
		}
	}

	return packageName, imports, nil
}

func IsGoBuildIgnore(comments []*ast.CommentGroup) bool {
	for _, comment := range comments {
		for _, line := range comment.List {
			if goBuildIgnoreRegex.Match([]byte(line.Text)) {
				return true
			}
		}
	}
	return false
}
