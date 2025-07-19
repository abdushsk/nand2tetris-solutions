package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
		filename := filepath.Base(fl)
		// Strip extension
		name := strings.TrimSuffix(filename, filepath.Ext(filename))

		for scanner.Scan() {
			line := scanner.Text()
			line = strings.TrimSpace(line)
			needed := ""
			slashC := 0
			for _, v := range line {
				if v == '/' {
					slashC++
					continue
				}
				if slashC == 2 {
					break
				}
				slashC = 0
				needed += string(v)
			}

			needed = strings.TrimSpace(needed)
			if len(needed) == 0 {
				continue
			}
			// fmt.Println(needed)
			assemblyLines[name] = append(assemblyLines[name], needed)
		}

		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading file:", err)
		}
	}

	code := []string{}

	if len(files) > 1 {
		code = []string{
			"@256",
			"D=A",
			"@SP",
			"M=D",
		}

		callCode := parseVmToAssembly("call Sys.init 0", "Sys")

		code = append(code, callCode...)
	}

	for filename, v := range assemblyLines {
		for i := range v {
			ret := parseVmToAssembly(v[i], filename)
			newR := []string{
				"//" + v[i],
			}
			newR = append(newR, ret...)
			code = append(code, newR...)
		}
	}

	baseName := ""

	if !path.IsDir() {
		slp := strings.Split(files[0], ".")
		baseName = strings.Join(slp[:len(slp)-1], ".")
	} else {
		name := os.Args[1]
		slp := strings.Split(name, "/")
		last := slp[len(slp)-1]
		baseName = filepath.Join(name, last)
	}

	newFileName := baseName + ".asm"
	os.WriteFile(newFileName, []byte(strings.Join(code, "\n")), 0755)

}

var index = 0
var retIndex = 0
var runningFunctionName = ""

func parseVmToAssembly(vm string, filename string) []string {
	vm = strings.TrimSpace(vm)

	splt := strings.Split(vm, " ")

	if len(splt) == 1 {
		if splt[0] == "add" || splt[0] == "sub" {
			sign := "+"
			if splt[0] == "sub" {
				sign = "-"
			}
			return []string{
				"@SP",
				"M=M-1",
				"A=M",
				"D=M",
				"@SP",
				"A=M-1",
				fmt.Sprintf("M=M%sD", sign),
			}
		}

		if splt[0] == "and" || splt[0] == "neg" || splt[0] == "or" || splt[0] == "not" {
			work := ""
			switch splt[0] {
			case "and":
				{
					work = "M&D"
				}
			case "or":
				{
					work = "M|D"
				}
			}

			if work == "" {
				if splt[0] == "neg" {
					return []string{
						"@SP",
						"A=M-1",
						"M=-M",
					}
				} else if splt[0] == "not" {
					return []string{
						"@SP",
						"A=M-1",
						"M=!M",
					}
				}
				panic("wtf??")
			}

			return []string{
				"@SP",
				"M=M-1",
				"A=M",
				"D=M",
				"@SP",
				"A=M-1",
				"M=" + work,
			}
		}

		if splt[0] == "lt" || splt[0] == "gt" || splt[0] == "eq" {
			code := ""
			switch splt[0] {
			case "gt":
				{
					code = "JGT"
				}
			case "lt":
				{
					code = "JLT"
				}
			case "eq":
				{
					code = "JEQ"
				}
			}

			int1 := index
			index++
			int2 := index
			index++

			return []string{
				"@SP",
				"M=M-1",
				"A=M",
				"D=M",
				"@SP",
				"A=M-1",
				"D=M-D",
				"M=0",
				fmt.Sprintf("@loopy.%d", int1),
				"D;" + code,
				fmt.Sprintf("@loopy.%d", int2),
				"0;JMP",
				fmt.Sprintf("(loopy.%d)", int1),
				"@SP",
				"A=M-1",
				"M=-1",
				fmt.Sprintf("(loopy.%d)", int2),
			}
		}

		if splt[0] == "return" {
			runningFunctionName = ""
			return []string{
				"@LCL",
				"D=M",
				"@5",
				"D=D-A",
				"A=D",
				"D=M",
				"@R14",
				"M=D",
				"@SP",
				"A=M-1",
				"D=M",
				"@ARG",
				"A=M",
				"M=D",
				"D=A",
				"@SP",
				"M=D+1",
				"@LCL",
				"D=M",
				"@1",
				"D=D-A",
				"A=D",
				"D=M",
				"@THAT",
				"M=D",

				"@LCL",
				"D=M",
				"@2",
				"D=D-A",
				"A=D",
				"D=M",
				"@THIS",
				"M=D",

				"@LCL",
				"D=M",
				"@3",
				"D=D-A",
				"A=D",
				"D=M",
				"@ARG",
				"M=D",

				"@LCL",
				"D=M",
				"@4",
				"D=D-A",
				"A=D",
				"D=M",
				"@LCL",
				"M=D",

				"@R14",
				"A=M",
				"0;JMP",
			}
		}

		fmt.Println(filename)
		fmt.Println(splt)
		panic(splt)
	}

	if splt[0] == "call" {
		funcName := splt[1]
		args, _ := strconv.Atoi(splt[2])
		returnLabel := fmt.Sprintf("%s.%s$ret.%d", filename, funcName, retIndex)
		retIndex++
		argStart := args + 5

		return []string{
			"@" + returnLabel,
			"D=A",
			"@SP",
			"M=M+1",
			"A=M-1",
			"M=D",
			"@LCL",
			"D=M",
			"@SP",
			"M=M+1",
			"A=M-1",
			"M=D",
			"@ARG",
			"D=M",
			"@SP",
			"M=M+1",
			"A=M-1",
			"M=D",
			"@THIS",
			"D=M",
			"@SP",
			"M=M+1",
			"A=M-1",
			"M=D",
			"@THAT",
			"D=M",
			"@SP",
			"M=M+1",
			"A=M-1",
			"M=D",
			fmt.Sprintf("@%d", argStart),
			"D=A",
			"@SP",
			"D=M-D",
			"@ARG",
			"M=D",
			"@SP",
			"D=M",
			"@LCL",
			"M=D",
			"@" + funcName,
			"0;JMP",
			fmt.Sprintf("(%s)", returnLabel),
		}
	}

	if splt[0] == "function" {
		funcName := splt[1]
		localArgs, _ := strconv.Atoi(splt[2])
		runningFunctionName = ""
		miniArgs := []string{
			fmt.Sprintf("(%s)", funcName),
		}

		for i := 0; i < localArgs; i++ {
			miniArgs = append(miniArgs, parseVmToAssembly("push constant 0", "")...)
		}
		return miniArgs
	}

	if splt[0] == "label" {
		name := splt[1]

		if runningFunctionName != "" {
			name = fmt.Sprintf("%s.%s$%s", filename, runningFunctionName, name)
		}

		return []string{
			fmt.Sprintf("(%s)", name),
		}
	}

	if splt[0] == "goto" {
		name := splt[1]
		if runningFunctionName != "" {
			name = fmt.Sprintf("%s.%s$%s", filename, runningFunctionName, name)
		}
		return []string{
			fmt.Sprintf("@%s", name),
			"0;JMP",
		}
	}

	if splt[0] == "if-goto" {
		name := splt[1]
		if runningFunctionName != "" {
			name = fmt.Sprintf("%s.%s$%s", filename, runningFunctionName, name)
		}
		return []string{
			"@SP",
			"M=M-1",
			"A=M",
			"D=M",
			fmt.Sprintf("@%s", name),
			"D;JNE",
		}
	}

	if splt[1] == "constant" {
		return []string{
			"@" + splt[2],
			"D=A",
			"@SP",
			"A=M",
			"M=D",
			"@SP",
			"M=M+1",
		}
	}

	if splt[1] == "temp" {
		num, _ := strconv.Atoi(splt[2])
		if splt[0] == "push" {
			return []string{
				"@" + fmt.Sprintf("%d", num+5),
				"D=M",
				"@SP",
				"A=M",
				"M=D",
				"@SP",
				"M=M+1",
			}
		} else {
			return []string{
				"@SP",
				"M=M-1",
				"A=M",
				"D=M",
				"@" + fmt.Sprintf("%d", num+5),
				"M=D",
			}
		}
	}

	if splt[1] == "static" {
		num, _ := strconv.Atoi(splt[2])
		loc := fmt.Sprintf("@%s.%d", filename, num)

		if splt[0] == "push" {
			return []string{
				loc,
				"D=M",
				"@SP",
				"M=M+1",
				"A=M-1",
				"M=D",
			}
		} else {
			return []string{
				"@SP",
				"M=M-1",
				"A=M",
				"D=M",
				loc,
				"M=D",
			}
		}
	}

	if splt[1] == "pointer" {
		num, _ := strconv.Atoi(splt[2])
		str := "THIS"
		if num == 1 {
			str = "THAT"
		}

		if splt[0] == "push" {
			return []string{
				"@" + str,
				"D=M",
				"@SP",
				"M=M+1",
				"A=M-1",
				"M=D",
			}
		} else {
			return []string{
				"@SP",
				"M=M-1",
				"A=M",
				"D=M",
				"@" + str,
				"M=D",
			}
		}

	}

	name := splt[1]

	switch name {
	case "local":
		{
			name = "LCL"
		}
	case "argument":
		{
			name = "ARG"
		}
	case "this":
		{
			name = "THIS"
		}
	case "that":
		{
			name = "THAT"
		}
	}

	if splt[0] == "push" {
		return []string{
			"@" + splt[2],
			"D=A",
			"@" + name,
			"A=D+M",
			"D=M",
			"@SP",
			"A=M",
			"M=D",
			"@SP",
			"M=M+1",
		}
	}

	// pop
	return []string{
		"@" + splt[2],
		"D=A",
		"@" + name,
		"D=M+D",
		"@R13",
		"M=D",
		"@SP",
		"M=M-1",
		"A=M",
		"D=M",
		"@R13",
		"A=M",
		"M=D",
	}
}
