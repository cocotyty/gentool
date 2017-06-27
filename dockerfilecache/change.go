package dockerfilecache

import (
	"bytes"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
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
func ReplaceDockerfileCache(pkg string, ignore map[string]bool) {
	ReplaceNamedDockerfileCache(pkg, ignore, "Dockerfile")
}

func ReplaceNamedDockerfileCache(pkg string, ignore map[string]bool, name string) {
	data, _ := ioutil.ReadFile(pathSrc + pkg + "/" + name)
	begin := bytes.Index(data, []byte("# GoGetBegin"))
	end := bytes.Index(data, []byte("# GoGetEnd"))
	if begin == -1 || end == -1 {
		panic("cannot find '# GoGetBegin' or '# GoGetEnd' ")
		return
	}
	pkgs := (&Context{pkg: pkg, ignore: ignore, imports: map[string]bool{}}).gen()

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

type Context struct {
	pkg     string
	ignore  map[string]bool
	imports map[string]bool
}

func (ctx *Context) gen() (pkgs []string) {
	pkgs = []string{}
	dirPath := pathSrc + ctx.pkg
	ctx.genDir(dirPath, map[string]bool{})
	for v := range ctx.imports {
		pkgs = append(pkgs, v)
	}
	sort.Strings(pkgs)
	return
}
func (ctx *Context) genDir(dirPath string, vendorPkgs map[string]bool) {
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
		vendorPkgs = copySet(vendorPkgs)
		findVendorPkg(dirPath, vendorPkgs)
		if f.IsDir() && f.Name() != "vendor" && !strings.HasPrefix(f.Name(), ".") {
			if !ctx.ignore[(dirPath + "/" + f.Name())[len(pathSrc):]] {
				ctx.genDir(dirPath+"/"+f.Name(), vendorPkgs)
			}
		} else {
			if strings.HasSuffix(f.Name(), ".go") && (!strings.HasSuffix(f.Name(), "_test.go")) {
				set := token.NewFileSet()
				f, err := parser.ParseFile(set, dirPath+"/"+f.Name(), nil, parser.ImportsOnly)
				if err != nil {
					continue
				}
				for _, im := range f.Imports {
					p, _ := strconv.Unquote(im.Path.Value)
					if !strings.HasPrefix(p, ctx.pkg) && (!standPkgs[strings.Split(p, "/")[0]]) && (!vendorPkgs[p]) {
						ctx.imports[p] = true
					}
				}
			}
		}
	}
}
func copySet(set map[string]bool) map[string]bool {
	copied := map[string]bool{}
	for k, v := range set {
		copied[k] = v
	}
	return copied
}
func findVendorPkg(dirPath string, set map[string]bool) {
	dirInfo, err := os.Lstat(dirPath + "/vendor")
	if err != nil {
		return
	}
	if !dirInfo.IsDir() {
		return
	}
	findValidPkg(set, dirPath+"/vendor/", "")
	return
}
func findValidPkg(set map[string]bool, base string, parent string) {
	dir, err := os.Open(base + parent)
	if err != nil {
		return
	}
	list, _ := dir.Readdir(-1)
	hasGoFile := false
	for _, v := range list {
		if v.IsDir() {
			if parent == "" {
				findValidPkg(set, base, v.Name())
			} else {
				findValidPkg(set, base, parent+"/"+v.Name())
			}
		}
		if (!v.IsDir()) && strings.HasSuffix(v.Name(), ".go") {
			hasGoFile = true
		}
	}
	if hasGoFile {
		set[parent] = true
	}
}
