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

func (si *SymbolInfo) Kindy() string {
	if si.Kind == "field" {
		return "this"
	}
	return si.Kind
}

type SymbolTable struct {
	Infos []*SymbolInfo
}

func (st *SymbolTable) Get(name string) *SymbolInfo {
	for _, v := range st.Infos {
		if v.Name == name {
			return v
		}
	}
	return nil
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
	nextLabelId      int
}

func (t *TokenAnnalyer) GetLabel() string {
	t.nextLabelId++
	return fmt.Sprintf("L_%d", t.nextLabelId)
}

func (t *TokenAnnalyer) getSymbol(name string) *SymbolInfo {
	name = strings.TrimPrefix(name, "this.")
	if t.subSymbolTable.Get(name) != nil {
		return t.subSymbolTable.Get(name)
	}
	if t.classSymbolTable.Get(name) != nil {
		return t.classSymbolTable.Get(name)
	}
	return t.ctxt.globalSymbolTable.Get(name)
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
		return strings.Join(t.parseClassToken(), "\n")
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

	if subKind == "method" {
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

	if subKind == "constructor" {
		classFieldCount := t.classSymbolTable.Count("field")
		tokens = append(tokens, []string{
			fmt.Sprintf("push constant %d", classFieldCount),
			"call Memory.alloc 1",
			"pop pointer 0",
		}...)
	}

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

func (t *TokenAnnalyer) getParamsCount() int {
	counter := map[string]int{
		"(": 1,
	}

	toTrack := []string{"(", "[", ")", "]"}

	commaFounds := 0
	i := t.currIndex
	for ; i < len(t.tokens); i++ {

		if funk.ContainsString(toTrack, t.tokens[i].Content) {
			if t.tokens[i].Content == ")" || t.tokens[i].Content == "]" {
				if t.tokens[i].Content == ")" {
					counter["("]--
				} else {
					counter["]"]--
				}
			} else {
				counter[t.tokens[i].Content]++
			}
		}

		nonZero := 0
		for _, v := range counter {
			if v > 0 {
				nonZero++
			}
		}

		if nonZero == 1 && t.tokens[i].Content == "," {
			commaFounds++
		}

		if nonZero == 0 {
			break
		}
	}

	if i == t.currIndex {
		return 0
	}

	return commaFounds + 1
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

	cnt := t.getParamsCount()

	if strings.Contains(name, ".") {
		symbolExists := t.getSymbol(strings.Split(name, ".")[0])
		if symbolExists != nil {
			tokens = append(tokens, []string{
				fmt.Sprintf("push %s %d", symbolExists.Kind, symbolExists.Index),
			}...)
			cnt++
			name = fmt.Sprintf("%s.%s", symbolExists.Type, strings.Split(name, ".")[1])
		}
	}

	tokens = append(tokens, t.parseExpressionList()...)

	tokens = append(tokens, fmt.Sprintf("call %s %d", name, cnt))
	tokens = append(tokens, "pop temp 0")

	t.smartAdvance(")", "")

	t.smartAdvance("", "symbol")
	return tokens
}

func (t *TokenAnnalyer) parseLetStatement() []string {
	tokens := []string{}

	t.smartAdvance("let", "keyword")
	variableNode := t.smartAdvance("", "identifier")
	foundSymbol := t.getSymbol(variableNode.Content)
	arrayMode := false
	nxt := t.get()
	if nxt.Content == "[" {
		arrayMode = true
		tokens = append(tokens, []string{
			fmt.Sprintf("push %s %d", foundSymbol.Kind, foundSymbol.Index),
		}...)

		t.smartAdvance("[", "")
		tokens = append(tokens, t.parseExpression()...)
		t.smartAdvance("]", "")

		tokens = append(tokens, "add")
	}

	t.smartAdvance("=", "symbol")
	tokens = append(tokens, t.parseExpression()...)
	t.smartAdvance(";", "symbol")

	if !arrayMode {
		tokens = append(tokens, fmt.Sprintf("pop %s %d", foundSymbol.Kindy(), foundSymbol.Index))
	} else {
		tokens = append(tokens, []string{
			"pop temp 0",
			"pop pointer 1",
			"push temp 0",
			"pop that 0",
		}...)
	}

	return tokens
}

func (t *TokenAnnalyer) parseIfStatement() []string {
	tokens := []string{}

	elseLabel := t.GetLabel()
	doneLabel := t.GetLabel()

	t.smartAdvance("if", "keyword")
	t.smartAdvance("(", "symbol")
	tokens = append(tokens, t.parseExpression()...)

	tokens = append(tokens, []string{
		"not",
		"if-goto " + elseLabel,
	}...)

	t.smartAdvance(")", "symbol")
	t.smartAdvance("{", "symbol")

	tokens = append(tokens, t.parseStatements()...)

	t.smartAdvance("}", "symbol")

	tokens = append(tokens, []string{
		"goto " + doneLabel,
		"label " + elseLabel,
	}...)

	tk := t.get()
	if tk.Content == "else" {
		t.smartAdvance("else", "")
		t.smartAdvance("{", "symbol")
		tokens = append(tokens, t.parseStatements()...)
		t.smartAdvance("}", "symbol")
	}

	tokens = append(tokens, []string{
		"label " + doneLabel,
	}...)

	return tokens
}

func (t *TokenAnnalyer) parseWhileStatement() []string {

	topmostLabel := t.GetLabel()
	breakingLabel := t.GetLabel()
	tokens := []string{
		"label " + topmostLabel,
	}

	t.smartAdvance("while", "keyword")
	t.smartAdvance("(", "symbol")
	tokens = append(tokens, t.parseExpression()...)
	tokens = append(tokens, []string{
		"not",
		"if-goto " + breakingLabel,
	}...)

	t.smartAdvance(")", "symbol")
	t.smartAdvance("{", "symbol")

	tokens = append(tokens, t.parseStatements()...)
	tokens = append(tokens, []string{
		"goto " + topmostLabel,
		"label " + breakingLabel,
	}...)

	t.smartAdvance("}", "symbol")

	return tokens
}

func (t *TokenAnnalyer) parseReturnStatement() []string {
	tokens := []string{}

	t.smartAdvance("return", "keyword")

	nxt := t.get()

	if nxt.Content != ";" {
		tokens = append(tokens, t.parseExpression()...)
	}
	t.smartAdvance(";", "")

	tokens = append(tokens, "return")
	return tokens
}

func convertOpToSomething(op string) string {
	switch op {
	case "*":
		return "call Math.multiply 2"
	case "/":
		return "call Math.divide 2"
	case "+":
		return "add"
	case "=":
		return "eq"
	case "-":
		return "sub"
	case "<":
		return "lt"
	case ">":
		return "gt"
	case "&":
		return "and"
	case "|":
		return "or"
	default:
		return op
	}
}

func (t *TokenAnnalyer) parseExpression() []string {
	tokens := []string{}
	tokens = append(tokens, t.parseTerm()...)

	ops := []string{"+", "-", "*", "/", "&", "|", "<", ">", "="}

	for {
		next := t.get()

		if funk.ContainsString(ops, next.Content) {
			fmt.Println("yuhu")
			opNode := t.smartAdvance("", "")
			tokens = append(tokens, t.parseTerm()...)
			tokens = append(tokens, convertOpToSomething(opNode.Content))
		} else {
			break
		}
	}

	return tokens
}

func (t *TokenAnnalyer) parseTerm() []string {
	tokens := []string{}

	tk1 := t.smartAdvance("", "")

	if tk1.Kind == "identifier" && t.getSymbol(tk1.Content) != nil {
		sym := t.getSymbol(tk1.Content)
		tokens = append(tokens, fmt.Sprintf("push %s %d", sym.Kind, sym.Index))
	}
	if tk1.Content == "null" {
		tokens = append(tokens, "push constant 0")
	}
	if tk1.Content == "true" {
		tokens = append(tokens, "push constant 1")
		tokens = append(tokens, "neg")
	}
	if tk1.Content == "false" {
		tokens = append(tokens, "push constant 0")
	}
	if tk1.Kind == "stringConstant" {
		stringLen := len(tk1.Content)
		tokens = append(tokens, []string{
			fmt.Sprintf("push constant %d", stringLen),
			"call String.new 1",
		}...)

		for _, v := range tk1.Content {
			tokens = append(tokens, []string{
				fmt.Sprintf("push constant %d", int(v)),
				"call String.appendChar 2",
			}...)
		}

	} else if tk1.Kind == "integerConstant" {
		tokens = append(tokens, fmt.Sprintf("push constant %s", tk1.Content))
	} else if tk1.Content == "-" || tk1.Content == "~" {
		tokens = append(tokens, t.parseTerm()...)
		btOp := "not"
		if tk1.Content == "-" {
			btOp = "neg"
		}
		tokens = append(tokens, btOp)
	} else if tk1.Content == "(" {
		// expression
		tokens = append(tokens, t.parseExpression()...)
		t.smartAdvance(")", "")
	} else {
		tk2 := t.get()

		if tk2.Content == "." {
			// var.sub-r call
			t.smartAdvance(".", "")
			name2Node := t.smartAdvance("", "identifier")
			t.smartAdvance("(", "")
			count := t.getParamsCount()
			tokens = append(tokens, t.parseExpressionList()...)
			tokens = append(tokens, fmt.Sprintf("call %s.%s %d", tk1.Content, name2Node.Content, count))
			t.smartAdvance(")", "")
		} else if tk2.Content == "[" {
			// array

			t.smartAdvance("[", "")

			tokens = append(tokens, t.parseExpression()...)
			tokens = append(tokens, "add")
			tokens = append(tokens, "pop pointer 1")
			tokens = append(tokens, "push that 0")

			t.smartAdvance("]", "")
		} else if tk2.Content == "(" {
			// sub-r call
			panic("kemm2")
		}
	}

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
			if idx == 0 {
				// panic("zupzup")
			}
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
	subroutines       []*SubroutineInfo
}

type SubroutineInfo struct {
	routineType string
	argsCount   int
	name        string
}

func (gc *GlobalContext) getRoutineInfo(name string, classname string) *SubroutineInfo {
	if !strings.Contains(name, ".") {
		name = classname + "." + name
	}

	for _, v := range gc.subroutines {
		if v.name == name {
			return v
		}
	}

	panic("yo?")
	return nil
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
			i++
			count := 1
			for ; i < len(tokens); i++ {
				if tokens[i].Content == ")" {
					break
				}
				if tokens[i].Content == "," {
					count++
				}
			}

			if tk == "method" {
				count++
			}

			gc.subroutines = append(gc.subroutines, &SubroutineInfo{
				routineType: tk,
				argsCount:   count,
				name:        name,
			})

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
