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
			if filepath.Ext(v.Name()) != ".jack" {
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

	tks := []*TokenAnnalyer{}

	ctxt := &GlobalContext{}

	for filename, v := range assemblyLines {
		tokens := []*Token{}
		for i := range v {
			tokens = append(tokens, parseLineToToken(v[i])...)
		}
		slp := strings.Split(filename, ".")
		baseName := strings.Join(slp[:len(slp)-1], ".")
		newFileName := baseName + "T.xml"
		tokens = joinTokens(tokens)
		os.WriteFile(newFileName, []byte(prettyPrintXML("<tokens>"+convertTokensToString(tokens)+"</tokens>")), 0755)

		t := TokenAnnalyer{
			tokens:    tokens,
			currIndex: 0,
			baseName:  baseName,
			ctxt:      ctxt,
		}

		ctxt.storeContext(tokens)
		tks = append(tks, &t)
	}

	for _, v := range tks {
		ret := v.run()
		newFileName := v.baseName + ".vm"
		// ret = strings.Replace(ret, "\n", "", -1)
		os.WriteFile(newFileName, []byte(ret), 0755)
	}
}

func removeEmptyLines(input string) string {
	lines := strings.Split(input, "\n")
	var nonEmpty []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	return strings.Join(nonEmpty, "\n")
}

func joinTokens(tokens []*Token) []*Token {
	newTokens := []*Token{}
	storedTkn := Token{}
	for _, v := range tokens {
		if v.Kind == "integerConstant" {
			storedTkn.Content = storedTkn.Content + v.Content
		} else {
			if storedTkn.Content != "" {
				newTokens = append(newTokens, &Token{
					Content: storedTkn.Content,
					Kind:    "integerConstant",
				})
			}

			storedTkn.Content = ""
			newTokens = append(newTokens, v)
		}
	}

	return newTokens
}

type SymbolInfo struct {
	Index int
	Kind  string
	Type  string
	Name  string
}

type SymbolTable struct {
	Infos []*SymbolInfo
}

func (st *SymbolTable) GetInfo(name string) *SymbolInfo {

	for _, v := range st.Infos {
		if v.Name == name {
			return v
		}
	}
	return nil
}

func (st *SymbolTable) Count(kind string) int {
	c := 0
	for _, v := range st.Infos {
		if v.Kind == kind {
			c++
		}
	}
	return c
}

func (st *SymbolTable) getNextInt(kind string) int {
	curr := -1
	for _, v := range st.Infos {
		if v.Kind == kind {
			curr++
		}
	}

	return curr + 1
}

func (s *SymbolTable) Reset() {
	s.Infos = []*SymbolInfo{}
}

type TokenAnnalyer struct {
	currIndex        int
	tokens           []*Token
	className        string
	subRoutineName   string
	subRoutineReturn string
	classSymbolTable SymbolTable
	subSymbolTable   SymbolTable
	ctxt             *GlobalContext
	baseName         string
}

func (t *TokenAnnalyer) GetContext() {

}

func (t *TokenAnnalyer) get() *Token {
	if t.currIndex >= len(t.tokens) {
		return nil
	}
	return t.tokens[t.currIndex]
}

func (t *TokenAnnalyer) smartAdvance(content string, kind string) *Token {
	tk := t.get()
	fmt.Println(tk)
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
	t.subSymbolTable.Reset()

	subKind := t.smartAdvance("", "keyword").Content
	t.subRoutineReturn = t.smartAdvance("", "").Content
	t.subRoutineName = t.smartAdvance("", "identifier").Content

	t.smartAdvance("(", "symbol")

	if subKind != "function" {
		t.subSymbolTable.Infos = append(t.subSymbolTable.Infos, &SymbolInfo{
			Index: 0,
			Kind:  "argument",
			Type:  t.className,
			Name:  "this",
		})
	}
	t.parseParameterList()

	t.smartAdvance(")", "symbol")

	ret := t.parseSubroutineBody()

	tokens := []string{}
	tokens = append(tokens, fmt.Sprintf("function %s.%s %d", t.className, t.subRoutineName, t.subSymbolTable.Count("local")))
	tokens = append(tokens, ret...)

	return tokens
}

func (t *TokenAnnalyer) parseSubroutineBody() []string {
	t.smartAdvance("{", "symbol")

	for {
		nxt := t.get()
		if nxt.Content == "var" {
			t.parseVarDec()
		} else {
			break
		}
	}

	tokens := []string{}
	tokens = append(tokens, t.parseStatements()...)
	t.smartAdvance("}", "symbol")
	return tokens
}

func (t *TokenAnnalyer) parseStatements() []string {
	tokens := []string{}

	for {
		nxt := t.get()
		if nxt.Content == "}" {
			break
		}
		if nxt.Content == "var" {
			panic("wtf??")
			t.parseVarDec()
		} else if nxt.Content == "let" {
			tokens = append(tokens, t.parseLetStatement()...)
		} else if nxt.Content == "do" {
			tokens = append(tokens, t.parseDoStatement()...)
		} else if nxt.Content == "return" {
			tokens = append(tokens, t.parseReturnStatement()...)
		} else if nxt.Content == "if" {
			tokens = append(tokens, t.parseIfStatement()...)
		} else if nxt.Content == "while" {
			tokens = append(tokens, t.parseWhileStatement()...)
		} else {
			fmt.Println(tokens)
			panic("here " + nxt.Content)
			break
		}
	}

	return tokens
}

func (t *TokenAnnalyer) parseDoStatement() []string {
	tokens := []string{}
	fmt.Println("yo?")
	t.smartAdvance("", "keyword")
	name := t.smartAdvance("", "").Content

	tk2 := t.smartAdvance("", "")

	if tk2.Content == "." {
		name += "." + t.smartAdvance("", "identifier").Content
		t.smartAdvance("(", "")
	}

	tokens = append(tokens, t.parseExpressionList()...)
	tokens = append(tokens, t.smartAdvance(")", "").String())

	tokens = append(tokens, t.smartAdvance("", "symbol").String())
	tokens = append(tokens, "</doStatement>")
	return tokens
}

func (t *TokenAnnalyer) parseLetStatement() []string {
	tokens := []string{
		"<letStatement>",
	}

	tokens = append(tokens, t.smartAdvance("", "keyword").String())
	tokens = append(tokens, t.smartAdvance("", "identifier").String())

	nxt := t.get()
	if nxt.Content == "[" {
		tokens = append(tokens, t.smartAdvance("[", "").String())
		tokens = append(tokens, t.parseExpression()...)
		tokens = append(tokens, t.smartAdvance("]", "").String())
	}

	tokens = append(tokens, t.smartAdvance("", "symbol").String())
	tokens = append(tokens, t.parseExpression()...)
	tokens = append(tokens, t.smartAdvance(";", "symbol").String())

	tokens = append(tokens, "</letStatement>")
	return tokens
}

func (t *TokenAnnalyer) parseIfStatement() []string {
	tokens := []string{
		"<ifStatement>",
	}

	tokens = append(tokens, t.smartAdvance("if", "keyword").String())
	tokens = append(tokens, t.smartAdvance("(", "symbol").String())
	tokens = append(tokens, t.parseExpression()...)
	fmt.Println(tokens)
	tokens = append(tokens, t.smartAdvance(")", "symbol").String())
	tokens = append(tokens, t.smartAdvance("{", "symbol").String())
	tokens = append(tokens, t.parseStatements()...)
	tokens = append(tokens, t.smartAdvance("}", "symbol").String())

	tk := t.get()
	if tk.Content == "else" {
		tokens = append(tokens, t.smartAdvance("else", "").String())
		tokens = append(tokens, t.smartAdvance("{", "symbol").String())
		tokens = append(tokens, t.parseStatements()...)
		tokens = append(tokens, t.smartAdvance("}", "symbol").String())
	}

	tokens = append(tokens, "</ifStatement>")
	return tokens
}

func (t *TokenAnnalyer) parseWhileStatement() []string {
	tokens := []string{
		"<whileStatement>",
	}

	tokens = append(tokens, t.smartAdvance("while", "keyword").String())
	tokens = append(tokens, t.smartAdvance("(", "symbol").String())
	tokens = append(tokens, t.parseExpression()...)
	tokens = append(tokens, t.smartAdvance(")", "symbol").String())
	tokens = append(tokens, t.smartAdvance("{", "symbol").String())
	tokens = append(tokens, t.parseStatements()...)
	tokens = append(tokens, t.smartAdvance("}", "symbol").String())

	tokens = append(tokens, "</whileStatement>")
	return tokens
}

func (t *TokenAnnalyer) parseReturnStatement() []string {
	tokens := []string{
		"<returnStatement>",
	}

	tokens = append(tokens, t.smartAdvance("", "keyword").String())

	nxt := t.get()

	if nxt.Content != ";" {
		tokens = append(tokens, t.parseExpression()...)
	}
	tokens = append(tokens, t.smartAdvance(";", "").String())

	tokens = append(tokens, "</returnStatement>")
	return tokens
}

func (t *TokenAnnalyer) parseExpression() []string {
	tokens := []string{
		"<expression>",
	}
	tokens = append(tokens, t.parseTerm()...)

	next := t.get()

	ops := []string{"+", "-", "*", "/", "&", "|", "<", ">", "="}

	if funk.ContainsString(ops, next.Content) {
		fmt.Println("yuhu")
		tokens = append(tokens, t.smartAdvance("", "").String())
		tokens = append(tokens, t.parseTerm()...)
	}

	tokens = append(tokens, "</expression>")
	return tokens
}

func (t *TokenAnnalyer) parseTerm() []string {
	tokens := []string{
		"<term>",
	}

	tk1 := t.smartAdvance("", "")
	tokens = append(tokens, tk1.String())

	if tk1.Content == "-" || tk1.Content == "~" {
		tokens = append(tokens, t.parseTerm()...)
	} else if tk1.Content == "(" {
		// expression
		tokens = append(tokens, t.parseExpression()...)
		tokens = append(tokens, t.smartAdvance(")", "").String())
	} else {
		tk2 := t.get()

		if tk2.Content == "." {
			// var.sub-r call
			tokens = append(tokens, t.smartAdvance("", "").String())
			tokens = append(tokens, t.smartAdvance("", "identifier").String())
			tokens = append(tokens, t.smartAdvance("(", "").String())
			tokens = append(tokens, t.parseExpressionList()...)
			tokens = append(tokens, t.smartAdvance(")", "").String())
		} else if tk2.Content == "[" {
			// array
			tokens = append(tokens, t.smartAdvance("[", "").String())
			tokens = append(tokens, t.parseExpression()...)
			tokens = append(tokens, t.smartAdvance("]", "").String())
		} else if tk2.Content == "(" {
			// sub-r call
			panic("kemm2")
		}
	}

	tokens = append(tokens, "</term>")
	return tokens
}

func (t *TokenAnnalyer) parseVarDec() {

	t.smartAdvance("", "keyword").String()
	typ := t.smartAdvance("", "").Content

	for {
		name := t.smartAdvance("", "identifier").Content

		t.subSymbolTable.Infos = append(t.subSymbolTable.Infos, &SymbolInfo{
			Name:  name,
			Index: t.subSymbolTable.getNextInt("local"),
			Kind:  "local",
			Type:  typ,
		})

		tk := t.smartAdvance(",", "")
		if tk == nil {
			break
		}
	}
	t.smartAdvance(";", "").String()
	return
}

func (t *TokenAnnalyer) parseClassVarDec() []string {
	tokens := []string{}

	kindNode := t.smartAdvance("", "keyword")

	typeNode := t.smartAdvance("", "")

	for {
		nameNode := t.smartAdvance("", "identifier")
		idx := 0

		if kindNode.Content == "static" {
			idx = t.ctxt.globalSymbolTable.GetInfo(t.className + "." + nameNode.Content).Index
		} else {
			idx = t.classSymbolTable.getNextInt(kindNode.Content)
		}

		t.classSymbolTable.Infos = append(t.classSymbolTable.Infos, &SymbolInfo{
			Kind:  kindNode.Content,
			Name:  nameNode.Content,
			Type:  typeNode.Content,
			Index: idx,
		})

		tk := t.smartAdvance(",", "")
		if tk == nil {
			break
		}
	}
	t.smartAdvance(";", "").String()
	return tokens
}

func (t *TokenAnnalyer) parseParameterList() {
	for {
		tk2 := t.get()
		if tk2.Content == ")" {
			break
		}

		typeNode := t.smartAdvance("", "").Content
		nameNode := t.smartAdvance("", "identifier").Content

		t.subSymbolTable.Infos = append(t.subSymbolTable.Infos, &SymbolInfo{
			Index: t.subSymbolTable.getNextInt("argument"),
			Name:  nameNode,
			Kind:  "argument",
			Type:  typeNode,
		})

		tk := t.smartAdvance(",", "")
		if tk == nil {
			break
		}
	}
}

func (t *TokenAnnalyer) parseExpressionList() []string {
	tokens := []string{}
	for {
		tk2 := t.get()
		if tk2.Content == ")" {
			break
		}

		tokens = append(tokens, t.parseExpression()...)
		tk := t.smartAdvance(",", "")
		if tk == nil {
			break
		}
	}
	return tokens
}

func (t *TokenAnnalyer) parseClassToken() []string {
	tokens := []string{}

	t.smartAdvance("class", "")
	t.className = t.smartAdvance("", "identifier").Content

	t.smartAdvance("{", "")
	for {
		next := t.get()
		if next.Content == "}" {
			t.smartAdvance("}", "")
			break
		}
		if next.Content == "constructor" || next.Content == "function" || next.Content == "method" {
			tokens = append(tokens, t.parseSubroutineDec()...)
		} else if next.Content == "static" || next.Content == "field" {
			tokens = append(tokens, t.parseClassVarDec()...)
		} else {
			fmt.Println("yos>")
			break
		}
	}

	t.className = ""
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
	if content == "&" {
		content = "&amp;"
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
		if v.Content == "&" {
			content = "&amp;"
		}
		str = append(str, fmt.Sprintf("<%s> %s </%s>", v.Kind, content, v.Kind))
	}

	return strings.Join(str, "")
}

type GlobalContext struct {
	globalSymbolTable SymbolTable
	allMethods        []string
	allConstructors   []string
	allFunctions      []string
}

func (gc *GlobalContext) storeContext(tokens []*Token) {
	className := ""

	for i := 0; i < len(tokens); i++ {

		tk := tokens[i].Content

		if tk == "class" {
			i++
			className = tokens[i].Content
			continue
		}

		if tk == "function" || tk == "constructor" || tk == "method" {
			i++
			i++
			name := className + "." + tokens[i].Content
			if tk == "function" {
				gc.allFunctions = append(gc.allFunctions, name)
			}
			if tk == "constructor" {
				gc.allConstructors = append(gc.allConstructors, name)
			}
			if tk == "method" {
				gc.allMethods = append(gc.allMethods, name)
			}
			continue
		}

		if tk == "static" {
			i++
			typ := tokens[i].Content
			i++
			for ; i < len(tokens); i++ {
				if tokens[i].Content == "," {
					continue
				}
				if tokens[i].Content == ";" {
					break
				}
				gc.globalSymbolTable.Infos = append(gc.globalSymbolTable.Infos, &SymbolInfo{
					Index: gc.globalSymbolTable.getNextInt("static"),
					Kind:  "static",
					Type:  typ,
					Name:  className + "." + tokens[i].Content,
				})
			}
			continue
		}

	}
}
