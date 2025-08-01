// This file is part of www.nand2tetris.org
// and the book "The Elements of Computing Systems"
// by Nisan and Schocken, MIT Press.
// File name: projects/4/Fill.asm

// Runs an infinite loop that listens to the keyboard input. 
// When a key is pressed (any key), the program blackens the screen,
// i.e. writes "black" in every pixel. When no key is pressed, 
// the screen should be cleared.

//// Replace this comment with your code.



(END)

@KBD
D=M

@val
M=0
@OUTER
D;JEQ

@val
M=-1

@OUTER
D;JNE

@END
0;JMP


(OUTER)
@SCREEN
D=A

@i
M=D

(LOOP)

@KBD
D=A

@i
D=M-D

@END
D;JEQ

@val
D=M

@i
A=M
M=D

@i
D=M+1
M=D

@LOOP
0;JMP