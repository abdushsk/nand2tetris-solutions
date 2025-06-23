package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var symbolTable map[string]int
var variableIndex int

func main() {
	fl := os.Args[1]

	file, err := os.Open(fl)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	assemblyLines := []string{}
	variableIndex = 16
	symbolTable = map[string]int{
		"R0":     0,
		"R1":     1,
		"R2":     2,
		"R3":     3,
		"R4":     4,
		"R5":     5,
		"R6":     6,
		"R7":     7,
		"R8":     8,
		"R9":     9,
		"R10":    10,
		"R11":    11,
		"R12":    12,
		"R13":    13,
		"R14":    14,
		"R15":    15,
		"SCREEN": 16384,
		"KBD":    24576,
		"SP":     0,
		"LCL":    1,
		"ARG":    2,
		"THIS":   3,
		"THAT":   4,
	}

	scanner := bufio.NewScanner(file)
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

		// assembly table shit

		if needed[0] == '(' {
			name := needed[1 : len(needed)-1]
			symbolTable[name] = len(assemblyLines)
			continue
		}

		assemblyLines = append(assemblyLines, needed)
	}

	fmt.Println(symbolTable)

	for i, v := range assemblyLines {
		assemblyLines[i] = parseStringToMl(v)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
	}

	newFileName := strings.Split(fl, ".")[0] + ".hack"
	os.WriteFile(newFileName, []byte(strings.Join(assemblyLines, "\n")), 0755)
}

func parseStringToMl(line string) string {
	if line[0] == '@' {
		name := line[1:]
		value, err := strconv.Atoi(name)

		if err != nil {
			_, f := symbolTable[name]
			if !f {
				symbolTable[name] = variableIndex
				variableIndex++
			}
			value = symbolTable[name]
		}

		binary := strconv.FormatInt(int64(value), 2)
		return fmt.Sprintf("0%015s", binary)
	} else {
		parts := strings.Split(line, "=")
		destStr := ""
		if len(parts) != 1 {
			destStr = parts[0]
		}
		divider := strings.Split(parts[len(parts)-1], ";")

		jump := ""
		if len(divider) != 1 {
			jump = divider[1]
		}
		binaryLive := "111" + conditionToControlBits(divider[0]) + destinationToBytes(destStr) + jumpToBytes(jump)
		if len(binaryLive) != 16 {
			panic("yooooooooooo " + binaryLive + "\n" + line)
		}
		return binaryLive
	}
}

func destinationToBytes(dest string) string {
	dest = strings.TrimSpace(dest)
	value := ""
	switch dest {
	case "":
		{
			value = "000"
		}
	case "M":
		{
			value = "001"
		}
	case "D":
		{
			value = "010"
		}
	case "DM":
		{
			value = "011"
		}
	case "MD":
		{
			value = "011"
		}
	case "A":
		{
			value = "100"
		}
	case "AM":
		{
			value = "101"
		}
	case "MA":
		{
			value = "101"
		}
	case "AD":
		{
			value = "110"
		}
	case "DA":
		{
			value = "110"
		}
	case "ADM":
		{
			value = "111"
		}
	case "AMD":
		{
			value = "111"
		}
	case "MAD":
		{
			value = "111"
		}
	case "MDA":
		{
			value = "111"
		}
	case "DMA":
		{
			value = "111"
		}
	case "DAM":
		{
			value = "111"
		}
	}

	if value == "" {
		panic(dest)
	}
	return value
}

func jumpToBytes(dest string) string {
	dest = strings.TrimSpace(dest)
	value := ""
	switch dest {
	case "":
		{
			value = "000"
		}
	case "JGT":
		{
			value = "001"
		}
	case "JEQ":
		{
			value = "101"
		}
	case "JGE":
		{
			value = "011"
		}
	case "JLT":
		{
			value = "100"
		}
	case "JNE":
		{
			value = "101"
		}
	case "JLE":
		{
			value = "110"
		}
	case "JMP":
		{
			value = "111"
		}
	}
	if value == "" {
		panic("yoo>")
	}
	return value
}

func conditionToControlBits(cond string) string {
	cond = strings.TrimSpace(cond)
	newCond := ""
	for _, v := range cond {
		if v == ' ' {
			continue
		}
		newCond = newCond + string(v)
	}

	z := map[string]string{
		"0":   "0101010",
		"1":   "0111111",
		"-1":  "0111010",
		"D":   "0001100",
		"A":   "0110000",
		"!D":  "0001101",
		"!A":  "0110001",
		"-D":  "0001111",
		"-A":  "0110011",
		"D+1": "0011111",
		"A+1": "0110111",
		"D-1": "0001110",
		"A-1": "0110010",
		"D+A": "0000010",
		"D-A": "0010011",
		"A-D": "0000111",
		"D&A": "0000000",
		"D|A": "0010101",
		"M":   "1110000",
		"!M":  "1110001",
		"-M":  "1110011",
		"M+1": "1110111",
		"M-1": "1110010",
		"D+M": "1000010",
		"D-M": "1010011",
		"M-D": "1000111",
		"D&M": "1000000",
		"D|M": "1010101",
	}

	if z[newCond] == "" {
		panic("yoooo")
	}

	return z[newCond]
}
