package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
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

func prettyPrintXML(input string) string {
	decoder := xml.NewDecoder(strings.NewReader(input))
	var out bytes.Buffer
	encoder := xml.NewEncoder(&out)
	encoder.Indent("", "  ") // set indentation

	for {
		tok, err := decoder.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			panic(err)
		}
		if err := encoder.EncodeToken(tok); err != nil {
			panic(err)
		}
	}
	encoder.Flush()
	return out.String()
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
		tokens := []*Token{}
		for i := range v {
			tokens = append(tokens, parseLineToToken(v[i])...)
		}
		slp := strings.Split(filename, ".")
		baseName := strings.Join(slp[:len(slp)-1], ".")
		newFileName := baseName + "T_m.xml"
		os.WriteFile(newFileName, []byte(prettyPrintXML("<tokens>"+convertTokensToString(tokens)+"</tokens>")), 0755)

		t := TokenAnnalyer{
			tokens:    tokens,
			currIndex: 0,
		}
		ret := t.run()
		newFileName = baseName + "F_m.xml"
		os.WriteFile(newFileName, []byte(prettyPrintXML(ret)), 0755)
	}
}

type TokenAnnalyer struct {
	currIndex int
	tokens    []*Token
}

func (t *TokenAnnalyer) get() *Token {
	if t.currIndex >= len(t.tokens) {
		return nil
	}
	return t.tokens[t.currIndex]
}

func (t *TokenAnnalyer) smartAdvance(content string, kind string) *Token {
	tk := t.get()
	if content != "" && tk.Content != content {
		return nil
	}

	if kind != "" && tk.Kind != kind {
		return nil
	}
	t.currIndex++
	return tk
}

func (t *TokenAnnalyer) run() string {
	tk := t.get()
	if tk.Content == "class" {
		return strings.Join(t.parseClassToken(), "")
	} else {
		panic("yo")
	}
}

func (t *TokenAnnalyer) parseSubroutineDec() []string {
	tokens := []string{
		"<subroutineDec>",
	}

	tokens = append(tokens, t.smartAdvance("", "keyword").String())
	tokens = append(tokens, t.smartAdvance("", "").String())
	tokens = append(tokens, t.smartAdvance("", "identifier").String())
	tokens = append(tokens, t.smartAdvance("(", "symbol").String())
	tokens = append(tokens, t.parseParameterList()...)
	tokens = append(tokens, t.smartAdvance(")", "symbol").String())
	tokens = append(tokens, t.parseSubroutineBody()...)
	tokens = append(tokens, "</subroutineDec>")
	return tokens
}

func (t *TokenAnnalyer) parseSubroutineBody() []string {
	tokens := []string{
		"<subroutineBody>",
	}
	tokens = append(tokens, t.smartAdvance("{", "symbol").String())

	for {
		nxt := t.get()
		if nxt.Content == "var" {
			tokens = append(tokens, t.parseVarDec()...)
		} else {
			break
		}
	}

	tokens = append(tokens, t.parseStatements()...)
	// tokens = append(tokens, t.smartAdvance("}", "symbol").String())
	tokens = append(tokens, "</subroutineBody>")
	return tokens
}

func (t *TokenAnnalyer) parseStatements() []string {
	tokens := []string{
		"<statements>",
	}

	for {
		nxt := t.get()
		if nxt.Content == "}" {
			break
		}
		if nxt.Content == "var" {
			tokens = append(tokens, t.parseVarDec()...)
		} else if nxt.Content == "let" {
			tokens = append(tokens, t.parseLetStatement()...)
		} else {
			break
		}
	}

	tokens = append(tokens, "</statements>")
	return tokens
}

func (t *TokenAnnalyer) parseLetStatement() []string {
	tokens := []string{
		"<letStatement>",
	}

	tokens = append(tokens, t.smartAdvance("", "keyword").String())
	tokens = append(tokens, t.smartAdvance("", "identifier").String())
	tokens = append(tokens, t.smartAdvance("", "symbol").String())
	tokens = append(tokens, t.smartAdvance("", "identifier").String())
	tokens = append(tokens, t.smartAdvance("", "symbol").String())

	tokens = append(tokens, "</letStatement>")
	return tokens
}

func (t *TokenAnnalyer) parseExpression() []string {
	tokens := []string{
		"<expression>",
	}

	tokens = append(tokens, "</expression>")
	return tokens
}

func (t *TokenAnnalyer) parseVarDec() []string {
	tokens := []string{
		"<varDec>",
	}

	tokens = append(tokens, t.smartAdvance("", "keyword").String())
	tokens = append(tokens, t.smartAdvance("", "").String())
	for {
		tokens = append(tokens, t.smartAdvance("", "identifier").String())
		tk := t.smartAdvance(",", "")
		if tk == nil {
			break
		}
		tokens = append(tokens, tk.String())
	}
	tokens = append(tokens, t.smartAdvance(";", "").String())

	tokens = append(tokens, "</varDec>")
	return tokens
}

func (t *TokenAnnalyer) parseClassVarDec() []string {
	tokens := []string{
		"<classVarDec>",
	}

	tokens = append(tokens, t.smartAdvance("", "keyword").String())
	tokens = append(tokens, t.smartAdvance("", "").String())
	for {
		tokens = append(tokens, t.smartAdvance("", "identifier").String())
		tk := t.smartAdvance(",", "")
		if tk == nil {
			break
		}
		tokens = append(tokens, tk.String())
	}
	tokens = append(tokens, t.smartAdvance(";", "").String())

	tokens = append(tokens, "</classVarDec>")
	return tokens
}

func (t *TokenAnnalyer) parseParameterList() []string {
	tokens := []string{
		"<parameterList>",
	}
	for {
		tk2 := t.get()
		if tk2.Content == ")" {
			break
		}

		tokens = append(tokens, t.smartAdvance("", "").String())
		tokens = append(tokens, t.smartAdvance("", "identifier").String())
		tk := t.smartAdvance(",", "")
		if tk == nil {
			break
		}
		tokens = append(tokens, tk.String())
	}
	tokens = append(tokens, "</parameterList>")
	return tokens
}

func (t *TokenAnnalyer) parseClassToken() []string {
	tokens := []string{
		"<class>",
	}

	tokens = append(tokens, t.smartAdvance("class", "").String())
	tokens = append(tokens, t.smartAdvance("", "identifier").String())
	tokens = append(tokens, t.smartAdvance("{", "").String())

	for {
		next := t.get()
		if next.Content == "}" {
			tokens = append(tokens, t.smartAdvance("}", "").String())
			break
		}
		if next.Content == "constructor" || next.Content == "function" || next.Content == "method" {
			tokens = append(tokens, t.parseSubroutineDec()...)
			break
		} else if next.Content == "static" || next.Content == "field" {
			tokens = append(tokens, t.parseClassVarDec()...)
		}
	}

	tokens = append(tokens, "</class>")

	return tokens
}

func parseLineToToken(line string) []*Token {
	ret := []*Token{}
	currW := ""
	stringStart := false
	for _, v := range line {
		if v == '"' {
			stringStart = !stringStart
			if !stringStart {
				ret = append(ret, NewToken("stringConstant", currW))
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
			ret = append(ret, NewToken("integerConstant", string(v)))
			continue
		}

		if funk.ContainsString(symbolTokens, string(v)) {
			if len(currW) != 0 {
				ret = append(ret, parseLineToToken(currW)...)
				currW = ""
			}
			ret = append(ret, NewToken("symbol", string(v)))
			continue
		}
		currW += string(v)
	}

	if len(currW) != 0 {
		if funk.ContainsString(keyboardTokens, currW) {
			ret = append(ret, NewToken("keyword", currW))
		} else {
			ret = append(ret, NewToken("identifier", currW))
		}
		currW = ""
	}

	return ret
}

type Token struct {
	Content string
	Kind    string
}

func (t *Token) String() string {
	return WrapString(t.Kind, t.Content)
}

func NewToken(wrapper string, content string) *Token {
	return &Token{
		Content: content,
		Kind:    wrapper,
	}
}

func WrapString(wrapper string, content string) string {
	if content == "<" {
		content = "&lt;"
	}
	if content == ">" {
		content = "&gt;"
	}
	return fmt.Sprintf("<%s> %s </%s>", wrapper, content, wrapper)
}

func convertTokensToString(tokens []*Token) string {
	str := []string{}
	for _, v := range tokens {
		content := v.Content
		if v.Content == "<" {
			content = "&lt;"
		}
		if v.Content == ">" {
			content = "&gt;"
		}
		str = append(str, fmt.Sprintf("<%s> %s </%s>", v.Kind, content, v.Kind))
	}

	return strings.Join(str, "")
}
