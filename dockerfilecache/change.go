package dockerfilecache

import (
	"sort"
	"os"
	"strings"
	"go/token"
	"go/parser"
	"strconv"
	"io/ioutil"
	"bytes"
)

var standPkgs = map[string]bool{}
var pathSrc string

func init() {
	goroot, _ := os.Open(os.Getenv("GOROOT") + "/src")
	rootDirs, _ := goroot.Readdir(-1)
	goroot.Close()
	for _, v := range rootDirs {
		standPkgs[v.Name()] = true
	}
	pathSrc = os.Getenv("GOPATH") + "/src/"
}
func ReplaceDockerfileCache(pkg string, ignore string) {
	data, _ := ioutil.ReadFile(pathSrc + pkg + "/Dockerfile")
	begin := bytes.Index(data, []byte("# GoGetBegin"))
	end := bytes.Index(data, []byte("# GoGetEnd"))
	pkgs := gen(pkg, ignore)

	buffer := bytes.NewBuffer(nil)
	buffer.Write(data[:begin])
	buffer.WriteString("# GoGetBegin\n")
	for _, p := range pkgs {
		buffer.WriteString("RUN go get ")
		buffer.WriteString(p)
		buffer.WriteByte('\n')
	}
	buffer.Write(data[end:])
	ioutil.WriteFile(pathSrc+pkg+"/Dockerfile", buffer.Bytes(), 0755)
}
func gen(pkg string, ignore string) (pkgs []string) {
	pkgs = []string{}
	dirPath := pathSrc + pkg
	m := map[string]bool{}
	genDir(pkg, dirPath, m, ignore)
	for v := range m {
		pkgs = append(pkgs, v)
	}
	sort.Strings(pkgs)
	return
}
func genDir(base, dirPath string, ctx map[string]bool, ignore string) {
	dir, err := os.Open(dirPath)
	if err != nil {
		panic(err)
	}
	fi, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		return
	}
	for _, f := range fi {
		if f.IsDir() {
			if pathSrc+ignore != dirPath+"/"+f.Name() {
				genDir(base, dirPath+"/"+f.Name(), ctx, ignore)
			}
		} else {
			if strings.HasSuffix(f.Name(), ".go") && (!strings.HasSuffix(f.Name(), "_test.go") ) {
				set := token.NewFileSet()
				f, err := parser.ParseFile(set, dirPath+"/"+f.Name(), nil, parser.ImportsOnly)
				if err != nil {
					continue
				}
				for _, im := range f.Imports {
					p, _ := strconv.Unquote(im.Path.Value)
					if !strings.HasPrefix(p, base) && (!standPkgs[strings.Split(p, "/")[0]]) {
						ctx[p] = true
					}
				}
			}
		}
	}
}
