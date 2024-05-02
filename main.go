package main

import (
	"bufio"
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
	// 项目根目录绝对路径
	projectDir := "/Users/jiangyang/project/pone"

	// 项目根目录下被修改文件的相对路径
	changeFileList := []string{
		"rpc/user/userclient/user.pb.go",
	}

	projectName, err := GetModuleName(projectDir)
	if err != nil {
		fmt.Println(err)
		return
	}

	packageTreeRoot := &PackageTree{PackageName: projectName, FilePath: projectDir, Children: make([]*PackageTree, 0)}
	if err := packageTreeRoot.Add(projectDir); err != nil {
		fmt.Println(err)
		return
	}

	packageTreeRoot.Print()
	checkList := Check(packageTreeRoot, changeFileList, projectName)
	fmt.Println("受影响的服务：", checkList)

}

// Check 找出fileList影响的服务
func Check(packageTreeRoot *PackageTree, fileList []string, projectName string) map[string]struct{} {
	packagesMap := make(map[string]struct{})
	for _, v := range fileList {
		if strings.HasSuffix(v, "_test.go") || !strings.HasSuffix(v, ".go") {
			continue
		}
		packages := packageTreeRoot.Find(packageTreeRoot, fmt.Sprintf("%s/%s", projectName, path.Dir(v)))
		for _, packagePath := range packages {
			packagesMap[packagePath] = struct{}{}
		}
	}
	return packagesMap
}

// Find 查找文件对应
func (t *PackageTree) Find(packageTreeRoot *PackageTree, packageName string) []string {
	affectedPackages := make([]string, 0)
	for _, v := range t.Imports {
		if v == packageName {
			if t.PackageName != "main" {
				affectedPackage := packageTreeRoot.Find(packageTreeRoot, fmt.Sprintf("%s%s", packageTreeRoot.PackageName, strings.TrimPrefix(t.FilePath, packageTreeRoot.FilePath)))
				affectedPackages = append(affectedPackages, affectedPackage...)
			} else {
				affectedPackages = append(affectedPackages, t.FilePath)
			}
		}
	}
	for _, v := range t.Children {
		affectedPackage := v.Find(packageTreeRoot, packageName)
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

var commendRegex = regexp.MustCompile(`^\s*//`)
var moduleRegex = regexp.MustCompile(`^\s*module\s+(\w+)`)

func GetModuleName(projectDir string) (string, error) {
	file, err := os.Open(fmt.Sprintf("%s/go.mod", projectDir))
	if err != nil {
		return "", err
	}
	buf := bufio.NewScanner(file)
	for buf.Scan() {
		if commendRegex.Match(buf.Bytes()) {
			continue
		}
		if match := moduleRegex.FindStringSubmatch(buf.Text()); len(match) > 1 && len(match[1]) > 0 {
			return match[1], nil
		}
	}
	return "", errors.New("module name not find")
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
				imports = append(imports, strings.Trim(v.Path.Value, `"`))
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
