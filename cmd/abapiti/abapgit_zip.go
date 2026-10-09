package main

import (
	"archive/zip"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// abapGitZipTime keeps the archive reproducible.
var abapGitZipTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// writeAbapGitZip packs ABAP sources named <obj>.clas.abap,
// <obj>.clas.testclasses.abap, <obj>.intf.abap and <obj>.prog.abap into an
// abapGit offline repository (prefix folder logic, everything in /src/).
func writeAbapGitZip(file, pkg, desc string, sources map[string]string) (int, error) {
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	files := map[string]string{
		".abapgit.xml": `<?xml version="1.0" encoding="utf-8"?>
<asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0">
 <asx:values>
  <DATA>
   <MASTER_LANGUAGE>E</MASTER_LANGUAGE>
   <STARTING_FOLDER>/src/</STARTING_FOLDER>
   <FOLDER_LOGIC>PREFIX</FOLDER_LOGIC>
  </DATA>
 </asx:values>
</asx:abap>
`,
		"src/package.devc.xml": abapGitXML("DEVC", "<DEVC>\n    <CTEXT>"+xmlText(truncate(desc+" ("+pkg+")", 60))+"</CTEXT>\n   </DEVC>"),
	}
	testclasses := map[string]bool{}
	for _, name := range names {
		if obj, ok := strings.CutSuffix(name, ".clas.testclasses.abap"); ok {
			testclasses[obj] = true
		}
	}
	objects := 0
	descr := xmlText(truncate(desc, 60))
	for _, name := range names {
		files["src/"+name] = sources[name]
		switch {
		case strings.HasSuffix(name, ".clas.abap"):
			obj := strings.TrimSuffix(name, ".clas.abap")
			unit := ""
			if testclasses[obj] {
				unit = "\n    <WITH_UNIT_TESTS>X</WITH_UNIT_TESTS>"
			}
			files["src/"+obj+".clas.xml"] = abapGitXML("CLAS", fmt.Sprintf("<VSEOCLASS>\n    <CLSNAME>%s</CLSNAME>\n    <LANGU>E</LANGU>\n    <DESCRIPT>%s</DESCRIPT>\n    <STATE>1</STATE>\n    <CLSCCINCL>X</CLSCCINCL>\n    <FIXPT>X</FIXPT>\n    <UNICODE>X</UNICODE>%s\n   </VSEOCLASS>", strings.ToUpper(obj), descr, unit))
			objects++
		case strings.HasSuffix(name, ".intf.abap"):
			obj := strings.TrimSuffix(name, ".intf.abap")
			files["src/"+obj+".intf.xml"] = abapGitXML("INTF", fmt.Sprintf("<VSEOINTERF>\n    <CLSNAME>%s</CLSNAME>\n    <LANGU>E</LANGU>\n    <DESCRIPT>%s</DESCRIPT>\n    <EXPOSURE>2</EXPOSURE>\n    <STATE>1</STATE>\n    <UNICODE>X</UNICODE>\n   </VSEOINTERF>", strings.ToUpper(obj), descr))
			objects++
		case strings.HasSuffix(name, ".prog.abap"):
			obj := strings.TrimSuffix(name, ".prog.abap")
			text := truncate(desc, 70)
			files["src/"+obj+".prog.xml"] = abapGitXML("PROG", fmt.Sprintf("<PROGDIR>\n    <NAME>%s</NAME>\n    <SUBC>1</SUBC>\n    <RSTAT>K</RSTAT>\n    <FIXPT>X</FIXPT>\n    <UCCHECK>X</UCCHECK>\n   </PROGDIR>\n   <TPOOL>\n    <item>\n     <ID>R</ID>\n     <ENTRY>%s</ENTRY>\n     <LENGTH>%d</LENGTH>\n    </item>\n   </TPOOL>", strings.ToUpper(obj), xmlText(text), len(text)))
			objects++
		case strings.HasSuffix(name, ".clas.testclasses.abap"):
		default:
			return 0, fmt.Errorf("abapGit zip: unsupported file %s", name)
		}
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	f, err := os.Create(file)
	if err != nil {
		return 0, err
	}
	w := zip.NewWriter(f)
	for _, p := range paths {
		zf, err := w.CreateHeader(&zip.FileHeader{Name: p, Method: zip.Deflate, Modified: abapGitZipTime})
		if err != nil {
			f.Close()
			return 0, err
		}
		if _, err := zf.Write([]byte(files[p])); err != nil {
			f.Close()
			return 0, err
		}
	}
	if err := w.Close(); err != nil {
		f.Close()
		return 0, err
	}
	return objects, f.Close()
}

func abapGitXML(kind, values string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<abapGit version="v1.0.0" serializer="LCL_OBJECT_` + kind + `" serializer_version="v1.0.0">
 <asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0">
  <asx:values>
   ` + values + `
  </asx:values>
 </asx:abap>
</abapGit>
`
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func xmlText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
