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
	"path/filepath"
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
	fmt.Println(os.Args)
	stat, err := os.Stdin.Stat()
	if err != nil {
		fmt.Println(err)
		return
	}
	if stat.Mode()&os.ModeNamedPipe != os.ModeNamedPipe {
		fmt.Println("没有标准输入")
		return
	}

	stdin := make([]string, 0)
	buf := bufio.NewScanner(os.Stdin)
	for buf.Scan() {
		stdin = append(stdin, buf.Text())
	}

	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
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

	//packageTreeRoot.Print()
	checkList := Check(packageTreeRoot, stdin)
	fmt.Println("受影响的服务：", strings.Join(checkList, ","))

}

// Check 找出fileList影响的服务
func Check(packageTreeRoot *PackageTree, fileList []string) []string {
	packagesMap := make(map[string]struct{})
	packageNames := make([]string, 0)
	for _, v := range fileList {
		if strings.HasSuffix(v, "_test.go") || !strings.HasSuffix(v, ".go") {
			continue
		}
		packages := packageTreeRoot.Find(packageTreeRoot, filepath.Join(packageTreeRoot.PackageName, path.Dir(v)))
		for _, packagePath := range packages {
			if _, ok := packagesMap[packagePath]; !ok {
				packagesMap[packagePath] = struct{}{}
				_, packageName := path.Split(packagePath)
				packageNames = append(packageNames, packageName)
			}
		}
	}
	return packageNames
}

// Find 递归查找文件影响的服务
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

// Print 打印依赖关系
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
			if err := t.Add(filepath.Join(dir, v.Name())); err != nil {
				return err
			}
		}
		if strings.HasSuffix(v.Name(), ".go") {
			fileList = append(fileList, filepath.Join(dir, v.Name()))
		}
	}
	if len(fileList) == 0 {
		return nil
	}

	packageName, imports, err := GetImportAndPackageName(fileList)
	if err != nil {
		return err
	}
	t.Children = append(t.Children, &PackageTree{PackageName: packageName, Imports: imports, FilePath: dir, Children: make([]*PackageTree, 0)})

	return nil
}

var commendRegex = regexp.MustCompile(`^\s*//`)
var moduleRegex = regexp.MustCompile(`^\s*module\s+(\w+)`)

// GetModuleName 获取项目模块名
func GetModuleName(projectDir string) (string, error) {
	file, err := os.Open(filepath.Join(projectDir, "go.mod"))
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

// GetImportAndPackageName 解析文件获取包名和依赖列表
func GetImportAndPackageName(filenames []string) (string, []string, error) {
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

// IsGoBuildIgnore 是否有 go:build或+build 编译约束
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
