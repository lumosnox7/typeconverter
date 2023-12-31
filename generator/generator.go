package generator

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lumosnox7/typeconverter/converter"
	"github.com/lumosnox7/typeconverter/expander"
)

type FolderName = string
type InterfaceName = string
type DefaultExportMap = map[FolderName]InterfaceName

type Generator struct {
	inputFolder      string // directory where all the go files with the structs are kept
	outputFolder     string // directory where all the ts files with the interfaces will go
	defaultExportMap DefaultExportMap
	expand           bool
}

func New(inputFolder string, outputFolder string, defaultExportMap DefaultExportMap, expand bool) (g *Generator) {
	return &Generator{
		inputFolder:      inputFolder,
		outputFolder:     outputFolder,
		defaultExportMap: defaultExportMap,
		expand:           expand,
	}
}

func (g *Generator) Loop() error {
	// create index file
	mainIndexPath := filepath.Join(g.outputFolder, "index.ts")

	if err := os.Mkdir(g.outputFolder, os.ModePerm); err != nil {
		return fmt.Errorf("creating folder: %v", err)
	}

	mainIndexFile, err := os.Create(mainIndexPath)
	if err != nil {
		return fmt.Errorf("creating file: %v", err)
	}
	mainIndexWriter := bufio.NewWriter(mainIndexFile)

	defaultExports := []string{}

	// loop through each folder
	folders, err := os.ReadDir(g.inputFolder)
	if err != nil {
		return fmt.Errorf("reading directory: %v", err)
	}
	for _, folder := range folders {
		if !folder.IsDir() || folder.Name() == ".git" || folder.Name() == g.outputFolder {
			continue
		}

		// create index file
		folderIndexPath := filepath.Join(g.outputFolder, folder.Name(), "index.ts")

		if err := os.Mkdir(filepath.Join(g.outputFolder, folder.Name()), os.ModePerm); err != nil {
			return fmt.Errorf("creating folder: %v", err)
		}

		folderIndexFile, err := os.Create(folderIndexPath)
		if err != nil {
			return fmt.Errorf("creating index file: %v", err)
		}
		folderIndexWriter := bufio.NewWriter(folderIndexFile)

		files, err := os.ReadDir(filepath.Join(g.inputFolder, folder.Name()))
		if err != nil {
			return fmt.Errorf("reading input directory: %v", err)
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}

			oldFile := filepath.Join(g.inputFolder, folder.Name(), file.Name())
			newFile := filepath.Join(g.outputFolder, folder.Name(), strings.Replace(file.Name(), ".go", ".ts", 1))

			if g.expand {
				// expand go structs to include update and filter structs
				expander.Expand(oldFile)
			}

			// convert go struct to ts interface
			res, err := converter.Convert(oldFile)
			if err != nil {
				fmt.Printf("converting structs: %v\n", err)
			}
			interfaces := res.Interfaces
			externalImports := res.ExternalImports
			internalImports := res.InternalImports
			fullText := res.FullText

			// make the import map
			importMap := make(map[string][]string)
			for _, i := range externalImports {
				inMap := false
				for pkg, structs := range importMap {
					if pkg == i.Package {
						inMap = true
						inStructs := false
						for _, str := range structs {
							if str == i.Struct {
								inStructs = true
							}
						}
						if !inStructs {
							importMap[pkg] = append(importMap[pkg], i.Struct)
						}
					}
				}
				if !inMap {
					importMap[i.Package] = []string{i.Struct}
				}
			}

			// create the new ts file
			newPath, newFN := filepath.Split(newFile)
			if err := os.MkdirAll(newPath, os.ModePerm); err != nil {
				return fmt.Errorf("creating file path: %v", err)
			}

			f, err := os.Create(newFile)
			if err != nil {
				return fmt.Errorf("creating file: %v", err)
			}

			// write the necessary imports to it
			w := bufio.NewWriter(f)
			for pkg, structs := range importMap {
				w.WriteString("import { ")
				for i, str := range structs {
					if i > 0 {
						w.WriteString(", ")
					}
					w.WriteString(str)
				}
				w.WriteString(" } from \"../" + pkg + "\";\n")
			}
			w.WriteString("\n")

			if len(internalImports) > 0 {
				w.WriteString("import { ")
				for i, str := range internalImports {
					if i > 0 {
						w.WriteString(", ")
					}
					w.WriteString(str)
				}
				w.WriteString(" } from \".\";\n")
				w.WriteString("\n")
			}

			// write the generated interface to it
			for _, s := range strings.Split(fullText, "\\n") {
				w.WriteString(s + "\n")
			}
			w.Flush()

			// write imports to folder index file
			defaultExport := ""
			de, ok := g.defaultExportMap[folder.Name()]
			importName := strings.ReplaceAll(strings.Replace("./"+newFN, ".ts", "", 1), string(filepath.Separator), "/")
			folderIndexWriter.WriteString("export { ")
			for i, interfaceName := range interfaces {
				if i > 0 {
					folderIndexWriter.WriteString(", ")
				}
				if ok && de == interfaceName {
					defaultExport = interfaceName
				}
				folderIndexWriter.WriteString(interfaceName)
			}
			folderIndexWriter.WriteString(" } from \"" + importName + "\";\n\n")
			folderIndexWriter.Flush()

			if defaultExport != "" {
				// write export to folder index file
				folderIndexWriter.WriteString("import { " + defaultExport + " } from \"" + importName + "\";\n")
				folderIndexWriter.WriteString("export default " + defaultExport + ";\n\n")
				folderIndexWriter.Flush()

				// write import to main index file
				mainIndexWriter.WriteString("import " + defaultExport + " from \"./" + folder.Name() + "\";\n")
				mainIndexWriter.Flush()
				defaultExports = append(defaultExports, defaultExport)
			}
		}
	}

	// write export to main index file
	mainIndexWriter.WriteString("\nexport { ")
	for i, de := range defaultExports {
		if i > 0 {
			mainIndexWriter.WriteString(", ")
		}
		mainIndexWriter.WriteString(de)
	}
	mainIndexWriter.WriteString(" }\n")
	mainIndexWriter.Flush()

	return nil
}
