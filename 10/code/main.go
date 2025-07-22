package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/thoas/go-funk"
)

var symbolTokens = []string{
	"{", "}", "(", ")", "[", "]", ".", ",", ";", "+", "-", "*", "/", "&", "|", "<", ">", "=", "~",
}

var keyboardTokens = []string{
	"class", "constructor", "function", "method", "field", "static", "var", "int", "char", "boolean", "void", "true", "false", "null", "this", "let", "do", "if", "else", "while", "return",
}

func main() {
	if len(os.Args) < 2 {
		panic("go run main.go <vm file>")
	}

	files := []string{}

	path, err := os.Stat(os.Args[1])
	if err != nil {
		panic(err)
	}

	if !path.IsDir() {
		files = append(files, os.Args[1])
	} else {
		entries, err := os.ReadDir(os.Args[1])
		if err != nil {
			panic(err)
		}

		for _, v := range entries {
			if filepath.Ext(v.Name()) != ".vm" {
				continue
			}

			files = append(files, filepath.Join(os.Args[1], v.Name()))
		}
	}

	assemblyLines := map[string][]string{}

	for _, fl := range files {
		file, err := os.Open(fl) // replace with your file path
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		// Strip extension

		cmdStarted := false

		for scanner.Scan() {
			line := scanner.Text()
			line = strings.TrimSpace(line)
			needed := line

			for {
				if cmdStarted {
					idx := strings.Index(needed, "*/")
					if idx != -1 {
						needed = line[idx+2:]
						cmdStarted = false
					} else {
						break
					}
				}

				newLongCmt := strings.Index(needed, "/**")
				if newLongCmt != -1 {
					cmdStarted = true
					continue
				} else {
					break
				}
			}

			if cmdStarted {
				continue
			}

			inclineCmt := strings.Index(needed, "//")
			if inclineCmt != -1 {
				needed = line[:inclineCmt]
			}

			needed = strings.TrimSpace(needed)
			if len(needed) == 0 {
				continue
			}
			fmt.Println(needed)
			assemblyLines[fl] = append(assemblyLines[fl], needed)
		}

		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading file:", err)
		}
	}

	for filename, v := range assemblyLines {
		tokens := []string{}
		for i := range v {
			tokens = append(tokens, parseLineToToken(v[i])...)
		}
		slp := strings.Split(filename, ".")
		baseName := strings.Join(slp[:len(slp)-1], ".")
		newFileName := baseName + "T_m.xml"
		os.WriteFile(newFileName, []byte(wrapString("tokens", "\n"+strings.Join(tokens, "\n")+"\n")), 0755)

		// parse tokens to final xml
	}
}

func parseLineToToken(line string) []string {
	ret := []string{}
	currW := ""
	stringStart := false
	for _, v := range line {
		if v == '"' {
			stringStart = !stringStart
			if !stringStart {
				ret = append(ret, wrapString("stringConstant", currW))
				currW = ""
			}
			continue
		}

		if stringStart {
			currW += string(v)
			continue
		}

		if v == ' ' {
			if len(currW) != 0 {
				ret = append(ret, parseLineToToken(currW)...)
				currW = ""
			}
			continue
		}

		_, err := strconv.Atoi(string(v))
		if err == nil && len(currW) == 0 {
			ret = append(ret, wrapString("integerConstant", string(v)))
			continue
		}

		if funk.ContainsString(symbolTokens, string(v)) {
			if len(currW) != 0 {
				ret = append(ret, parseLineToToken(currW)...)
				currW = ""
			}
			ret = append(ret, wrapString("symbol", string(v)))
			continue
		}
		currW += string(v)
	}

	if len(currW) != 0 {
		if funk.ContainsString(keyboardTokens, currW) {
			ret = append(ret, wrapString("keyword", currW))
		} else {
			ret = append(ret, wrapString("identifier", currW))
		}
		currW = ""
	}

	return ret
}

func wrapString(wrapper string, content string) string {
	if content == "<" {
		content = "&lt;"
	}
	if content == ">" {
		content = "&gt;"
	}
	return fmt.Sprintf("<%s> %s </%s>", wrapper, content, wrapper)
}
