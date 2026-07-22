TEXT github.com/gregriff/vogo/cli/internal/audio.(*streams).mixAVX512(SB) C:/Users/gregg/code/vogo/cli/internal/audio/mixing_amd64.go
  mixing_amd64.go:9		0x14054d2c0		4c8da42418f8ffff		LEAQ 0xfffff818(SP), R12							
  mixing_amd64.go:9		0x14054d2c8		4d3b6610			CMPQ R12, 0x10(R14)								
  mixing_amd64.go:9		0x14054d2cc		0f86ac050000			JBE 0x14054d87e									
  mixing_amd64.go:9		0x14054d2d2		55				PUSHQ BP									
  mixing_amd64.go:9		0x14054d2d3		4889e5				MOVQ SP, BP									
  mixing_amd64.go:9		0x14054d2d6		4881ec60080000			SUBQ $0x860, SP									
  mixing_amd64.go:13		0x14054d2dd		4889842470080000		MOVQ AX, 0x870(SP)								
  mixing_amd64.go:13		0x14054d2e5		48899c2478080000		MOVQ BX, 0x878(SP)								
  mixing_amd64.go:11		0x14054d2ed		488d9424d0070000		LEAQ 0x7d0(SP), DX								
  mixing_amd64.go:11		0x14054d2f5		440f113a			MOVUPS X15, 0(DX)								
  mixing_amd64.go:11		0x14054d2f9		440f117a10			MOVUPS X15, 0x10(DX)								
  mixing_amd64.go:11		0x14054d2fe		440f117a18			MOVUPS X15, 0x18(DX)								
  mixing_amd64.go:13		0x14054d303		488b5808			MOVQ 0x8(AX), BX								
  mixing_amd64.go:13		0x14054d307		488d8c2400080000		LEAQ 0x800(SP), CX								
  mixing_amd64.go:13		0x14054d30f		440f1139			MOVUPS X15, 0(CX)								
  mixing_amd64.go:13		0x14054d313		440f117910			MOVUPS X15, 0x10(CX)								
  mixing_amd64.go:13		0x14054d318		440f117920			MOVUPS X15, 0x20(CX)								
  mixing_amd64.go:13		0x14054d31d		440f117930			MOVUPS X15, 0x30(CX)								
  mixing_amd64.go:13		0x14054d322		440f117940			MOVUPS X15, 0x40(CX)								
  mixing_amd64.go:13		0x14054d327		440f117950			MOVUPS X15, 0x50(CX)								
  mixing_amd64.go:13		0x14054d32c		488d05e5867600			LEAQ type:*+452400(SB), AX							
  mixing_amd64.go:13		0x14054d333		e86855adff			CALL runtime.mapIterStart(SB)							
  mixing_amd64.go:13		0x14054d338		31c0				XORL AX, AX									
  mixing_amd64.go:13		0x14054d33a		eb1b				JMP 0x14054d357									
  mixing_amd64.go:13		0x14054d33c		8984249c070000			MOVL AX, 0x79c(SP)								
  mixing_amd64.go:13		0x14054d343		488d842400080000		LEAQ 0x800(SP), AX								
  mixing_amd64.go:13		0x14054d34b		e8b055adff			CALL runtime.mapIterNext(SB)							
  mixing_amd64.go:13		0x14054d350		8b84249c070000			MOVL 0x79c(SP), AX								
  mixing_amd64.go:13		0x14054d357		8984249c070000			MOVL AX, 0x79c(SP)								
  mixing_amd64.go:13		0x14054d35e		4883bc240008000000		CMPQ 0x800(SP), $0x0								
  mixing_amd64.go:13		0x14054d367		0f848b020000			JE 0x14054d5f8									
  mixing_amd64.go:13		0x14054d36d		488b942408080000		MOVQ 0x808(SP), DX								
  mixing_amd64.go:13		0x14054d375		488b12				MOVQ 0(DX), DX									
  ringbuffer.go:85		0x14054d378		488b7228			MOVQ 0x28(DX), SI								
  mixing_amd64.go:14		0x14054d37c		488bbc2478080000		MOVQ 0x878(SP), DI								
  mixing_amd64.go:14		0x14054d384		4839f7				CMPQ DI, SI									
  mixing_amd64.go:14		0x14054d387		7fb3				JG 0x14054d33c									
  mixing_amd64.go:16		0x14054d389		4c63c0				MOVSXD AX, R8									
  mixing_amd64.go:16		0x14054d38c		4983f805			CMPQ R8, $0x5									
  mixing_amd64.go:16		0x14054d390		0f83e2040000			JAE 0x14054d878									
  mixing_amd64.go:16		0x14054d396		4d69c8c0030000			IMULQ $0x3c0, R8, R9								
  mixing_amd64.go:16		0x14054d39d		4c8b942470080000		MOVQ 0x870(SP), R10								
  mixing_amd64.go:16		0x14054d3a5		4f8d1c0a			LEAQ 0(R10)(R9*1), R11								
  mixing_amd64.go:16		0x14054d3a9		4d8d5b10			LEAQ 0x10(R11), R11								
  ringbuffer.go:66		0x14054d3ad		4881fee0010000			CMPQ SI, $0x1e0									
  ringbuffer.go:67		0x14054d3b4		41bce0010000			MOVL $.file+403(SB), R12							
  ringbuffer.go:67		0x14054d3ba		490f4ff4			CMOVG R12, SI									
  ringbuffer.go:67		0x14054d3be		6690				NOPW										
  ringbuffer.go:67		0x14054d3c0		4885f6				TESTQ SI, SI									
  ringbuffer.go:66		0x14054d3c3		7508				JNE 0x14054d3cd									
  mixing_amd64.go:20		0x14054d3c5		4189c3				MOVL AX, R11									
  mixing_amd64.go:16		0x14054d3c8		e912020000			JMP 0x14054d5df									
  mixing_amd64.go:13		0x14054d3cd		48899424c8070000		MOVQ DX, 0x7c8(SP)								
  mixing_amd64.go:16		0x14054d3d5		4c898424c0070000		MOVQ R8, 0x7c0(SP)								
  ringbuffer.go:67		0x14054d3dd		4889b424a0070000		MOVQ SI, 0x7a0(SP)								
  mixing_amd64.go:16		0x14054d3e5		4c898c24b8070000		MOVQ R9, 0x7b8(SP)								
  ringbuffer.go:71		0x14054d3ed		4c8b6a30			MOVQ 0x30(DX), R13								
  ringbuffer.go:71		0x14054d3f1		4c8b7a18			MOVQ 0x18(DX), R15								
  ringbuffer.go:71		0x14054d3f5		4d29fd				SUBQ R15, R13									
  ringbuffer.go:71		0x14054d3f8		0f1f840000000000		NOPL 0(AX)(AX*1)								
  ringbuffer.go:72		0x14054d400		4c39ee				CMPQ SI, R13									
  ringbuffer.go:72		0x14054d403		0f8f90000000			JG 0x14054d499									
  ringbuffer.go:73		0x14054d409		4c8b6a10			MOVQ 0x10(DX), R13								
  ringbuffer.go:73		0x14054d40d		498d0c37			LEAQ 0(R15)(SI*1), CX								
  ringbuffer.go:73		0x14054d411		4939cd				CMPQ R13, CX									
  ringbuffer.go:73		0x14054d414		0f8259040000			JB 0x14054d873									
  ringbuffer.go:73		0x14054d41a		660f1f440000			NOPW 0(AX)(AX*1)								
  ringbuffer.go:73		0x14054d420		4939cf				CMPQ R15, CX									
  ringbuffer.go:73		0x14054d423		0f8745040000			JA 0x14054d86e									
  ringbuffer.go:73		0x14054d429		488b0a				MOVQ 0(DX), CX									
  ringbuffer.go:73		0x14054d42c		4b8d1c3f			LEAQ 0(R15)(R15*1), BX								
  ringbuffer.go:73		0x14054d430		4d29ef				SUBQ R13, R15									
  ringbuffer.go:73		0x14054d433		49c1ff3f			SARQ $0x3f, R15									
  ringbuffer.go:73		0x14054d437		4c21fb				ANDQ R15, BX									
  ringbuffer.go:73		0x14054d43a		4801cb				ADDQ CX, BX									
  ringbuffer.go:73		0x14054d43d		4881fee0010000			CMPQ SI, $0x1e0									
  ringbuffer.go:73		0x14054d444		4c0f4ce6			CMOVL SI, R12									
  ringbuffer.go:73		0x14054d448		4939db				CMPQ R11, BX									
  ringbuffer.go:73		0x14054d44b		0f8457010000			JE 0x14054d5a8									
  ringbuffer.go:73		0x14054d451		4b8d0c24			LEAQ 0(R12)(R12*1), CX								
  ringbuffer.go:73		0x14054d455		4c89d8				MOVQ R11, AX									
  ringbuffer.go:73		0x14054d458		e8c336b4ff			CALL runtime.memmove(SB)							
  mixing_amd64.go:20		0x14054d45d		8b84249c070000			MOVL 0x79c(SP), AX								
  ringbuffer.go:79		0x14054d464		488b9424c8070000		MOVQ 0x7c8(SP), DX								
  ringbuffer.go:79		0x14054d46c		488bb424a0070000		MOVQ 0x7a0(SP), SI								
  mixing_amd64.go:14		0x14054d474		488bbc2478080000		MOVQ 0x878(SP), DI								
  mixing_amd64.go:19		0x14054d47c		4c8b8424c0070000		MOVQ 0x7c0(SP), R8								
  mixing_amd64.go:19		0x14054d484		4c8b8c24b8070000		MOVQ 0x7b8(SP), R9								
  mixing_amd64.go:19		0x14054d48c		4c8b942470080000		MOVQ 0x870(SP), R10								
  ringbuffer.go:73		0x14054d494		e90f010000			JMP 0x14054d5a8									
  ringbuffer.go:75		0x14054d499		488b4a08			MOVQ 0x8(DX), CX								
  ringbuffer.go:75		0x14054d49d		0f1f00				NOPL 0(AX)									
  ringbuffer.go:75		0x14054d4a0		4c39f9				CMPQ CX, R15									
  ringbuffer.go:75		0x14054d4a3		0f82c0030000			JB 0x14054d869									
  ringbuffer.go:75		0x14054d4a9		488b1a				MOVQ 0(DX), BX									
  ringbuffer.go:75		0x14054d4ac		488b7a10			MOVQ 0x10(DX), DI								
  ringbuffer.go:75		0x14054d4b0		4c29f9				SUBQ R15, CX									
  ringbuffer.go:75		0x14054d4b3		4b8d043f			LEAQ 0(R15)(R15*1), AX								
  ringbuffer.go:75		0x14054d4b7		4929ff				SUBQ DI, R15									
  ringbuffer.go:75		0x14054d4ba		49c1ff3f			SARQ $0x3f, R15									
  ringbuffer.go:75		0x14054d4be		4c21f8				ANDQ R15, AX									
  ringbuffer.go:75		0x14054d4c1		4801c3				ADDQ AX, BX									
  ringbuffer.go:75		0x14054d4c4		4881f9e0010000			CMPQ CX, $0x1e0									
  ringbuffer.go:75		0x14054d4cb		4c0f4ce1			CMOVL CX, R12									
  ringbuffer.go:75		0x14054d4cf		4939db				CMPQ R11, BX									
  ringbuffer.go:75		0x14054d4d2		7454				JE 0x14054d528									
  mixing_amd64.go:16		0x14054d4d4		4c899c24f8070000		MOVQ R11, 0x7f8(SP)								
  ringbuffer.go:71		0x14054d4dc		4c89ac24a8070000		MOVQ R13, 0x7a8(SP)								
  ringbuffer.go:75		0x14054d4e4		4b8d0c24			LEAQ 0(R12)(R12*1), CX								
  ringbuffer.go:75		0x14054d4e8		4c89d8				MOVQ R11, AX									
  ringbuffer.go:75		0x14054d4eb		e83036b4ff			CALL runtime.memmove(SB)							
  ringbuffer.go:76		0x14054d4f0		488b9424c8070000		MOVQ 0x7c8(SP), DX								
  ringbuffer.go:76		0x14054d4f8		488bb424a0070000		MOVQ 0x7a0(SP), SI								
  mixing_amd64.go:19		0x14054d500		4c8b8424c0070000		MOVQ 0x7c0(SP), R8								
  mixing_amd64.go:19		0x14054d508		4c8b8c24b8070000		MOVQ 0x7b8(SP), R9								
  mixing_amd64.go:19		0x14054d510		4c8b942470080000		MOVQ 0x870(SP), R10								
  ringbuffer.go:76		0x14054d518		4c8b9c24f8070000		MOVQ 0x7f8(SP), R11								
  ringbuffer.go:76		0x14054d520		4c8bac24a8070000		MOVQ 0x7a8(SP), R13								
  ringbuffer.go:76		0x14054d528		4981fde0010000			CMPQ R13, $0x1e0								
  ringbuffer.go:76		0x14054d52f		0f872a030000			JA 0x14054d85f									
  ringbuffer.go:76		0x14054d535		488b7a10			MOVQ 0x10(DX), DI								
  ringbuffer.go:76		0x14054d539		4989f4				MOVQ SI, R12									
  ringbuffer.go:76		0x14054d53c		4c29ee				SUBQ R13, SI									
  ringbuffer.go:76		0x14054d53f		4b8d046b			LEAQ 0(R11)(R13*2), AX								
  ringbuffer.go:76		0x14054d543		4839f7				CMPQ DI, SI									
  ringbuffer.go:76		0x14054d546		0f820e030000			JB 0x14054d85a									
  ringbuffer.go:76		0x14054d54c		498dbd20feffff			LEAQ 0xfffffe20(R13), DI							
  ringbuffer.go:76		0x14054d553		48f7df				NEGQ DI										
  ringbuffer.go:76		0x14054d556		488b1a				MOVQ 0(DX), BX									
  ringbuffer.go:76		0x14054d559		4839f7				CMPQ DI, SI									
  ringbuffer.go:76		0x14054d55c		480f4ffe			CMOVG SI, DI									
  ringbuffer.go:76		0x14054d560		4839c3				CMPQ BX, AX									
  ringbuffer.go:76		0x14054d563		7431				JE 0x14054d596									
  ringbuffer.go:76		0x14054d565		488d0c3f			LEAQ 0(DI)(DI*1), CX								
  ringbuffer.go:76		0x14054d569		e8b235b4ff			CALL runtime.memmove(SB)							
  ringbuffer.go:79		0x14054d56e		488b9424c8070000		MOVQ 0x7c8(SP), DX								
  mixing_amd64.go:19		0x14054d576		4c8b8424c0070000		MOVQ 0x7c0(SP), R8								
  mixing_amd64.go:19		0x14054d57e		4c8b8c24b8070000		MOVQ 0x7b8(SP), R9								
  mixing_amd64.go:19		0x14054d586		4c8b942470080000		MOVQ 0x870(SP), R10								
  ringbuffer.go:79		0x14054d58e		4c8ba424a0070000		MOVQ 0x7a0(SP), R12								
  mixing_amd64.go:20		0x14054d596		8b84249c070000			MOVL 0x79c(SP), AX								
  ringbuffer.go:79		0x14054d59d		4c89e6				MOVQ R12, SI									
  mixing_amd64.go:14		0x14054d5a0		488bbc2478080000		MOVQ 0x878(SP), DI								
  ringbuffer.go:79		0x14054d5a8		488b4a18			MOVQ 0x18(DX), CX								
  ringbuffer.go:79		0x14054d5ac		488b5a30			MOVQ 0x30(DX), BX								
  ringbuffer.go:79		0x14054d5b0		4801f1				ADDQ SI, CX									
  ringbuffer.go:79		0x14054d5b3		4885db				TESTQ BX, BX									
  ringbuffer.go:79		0x14054d5b6		0f8499020000			JE 0x14054d855									
  mixing_amd64.go:13		0x14054d5bc		4189c3				MOVL AX, R11									
  ringbuffer.go:79		0x14054d5bf		4889c8				MOVQ CX, AX									
  mixing_amd64.go:13		0x14054d5c2		4889d1				MOVQ DX, CX									
  ringbuffer.go:79		0x14054d5c5		4883fbff			CMPQ BX, $-0x1									
  ringbuffer.go:79		0x14054d5c9		7507				JNE 0x14054d5d2									
  ringbuffer.go:79		0x14054d5cb		48f7d8				NEGQ AX										
  ringbuffer.go:79		0x14054d5ce		31d2				XORL DX, DX									
  ringbuffer.go:79		0x14054d5d0		eb05				JMP 0x14054d5d7									
  ringbuffer.go:79		0x14054d5d2		4899				CQO										
  ringbuffer.go:79		0x14054d5d4		48f7fb				IDIVQ BX									
  ringbuffer.go:79		0x14054d5d7		48895118			MOVQ DX, 0x18(CX)								
  ringbuffer.go:80		0x14054d5db		48297128			SUBQ SI, 0x28(CX)								
  mixing_amd64.go:19		0x14054d5df		4b8d0c0a			LEAQ 0(R10)(R9*1), CX								
  mixing_amd64.go:19		0x14054d5e3		488d4910			LEAQ 0x10(CX), CX								
  mixing_amd64.go:19		0x14054d5e7		4a898cc4d0070000		MOVQ CX, 0x7d0(SP)(R8*8)							
  mixing_amd64.go:20		0x14054d5ef		418d4301			LEAL 0x1(R11), AX								
  mixing_amd64.go:20		0x14054d5f3		e944fdffff			JMP 0x14054d33c									
  mixing_amd64.go:25		0x14054d5f8		488b9c2470080000		MOVQ 0x870(SP), BX								
  mixing_amd64.go:25		0x14054d600		488d93d0120000			LEAQ 0x12d0(BX), DX								
  mixing_amd64.go:25		0x14054d607		4889d1				MOVQ DX, CX									
  mixing_amd64.go:25		0x14054d60a		be0f000000			MOVL $__major_subsystem_version__+5(SB), SI					
  mixing_amd64.go:25		0x14054d60f		440f113a			MOVUPS X15, 0(DX)								
  mixing_amd64.go:25		0x14054d613		440f117a10			MOVUPS X15, 0x10(DX)								
  mixing_amd64.go:25		0x14054d618		440f117a20			MOVUPS X15, 0x20(DX)								
  mixing_amd64.go:25		0x14054d61d		440f117a30			MOVUPS X15, 0x30(DX)								
  mixing_amd64.go:25		0x14054d622		4883c240			ADDQ $0x40, DX									
  mixing_amd64.go:25		0x14054d626		ffce				DECL SI										
  mixing_amd64.go:25		0x14054d628		75e5				JNE 0x14054d60f									
  mixing_amd64.go:26		0x14054d62a		85c0				TESTL AX, AX									
  mixing_amd64.go:26		0x14054d62c		0f84b3000000			JE 0x14054d6e5									
  mixing_amd64.go:26		0x14054d632		488b5308			MOVQ 0x8(BX), DX								
  mixing_amd64.go:26		0x14054d636		4885d2				TESTQ DX, DX									
  mixing_amd64.go:26		0x14054d639		7407				JE 0x14054d642									
  mixing_amd64.go:26		0x14054d63b		488b12				MOVQ 0(DX), DX									
  mixing_amd64.go:26		0x14054d63e		6690				NOPW										
  mixing_amd64.go:26		0x14054d640		eb02				JMP 0x14054d644									
  mixing_amd64.go:26		0x14054d642		31d2				XORL DX, DX									
  mixing_amd64.go:26		0x14054d644		4885d2				TESTQ DX, DX									
  mixing_amd64.go:26		0x14054d647		0f8498000000			JE 0x14054d6e5									
  mixing_amd64.go:31		0x14054d64d		83f801				CMPL AX, $0x1									
  mixing_amd64.go:31		0x14054d650		7451				JE 0x14054d6a3									
  mixing_amd64.go:37		0x14054d652		488d7c241c			LEAQ 0x1c(SP), DI								
  mixing_amd64.go:37		0x14054d657		b9f0000000			MOVL $.file+163(SB), CX								
  mixing_amd64.go:13		0x14054d65c		89c2				MOVL AX, DX									
  mixing_amd64.go:37		0x14054d65e		31c0				XORL AX, AX									
  mixing_amd64.go:37		0x14054d660		f348ab				REP; STOSQ AX, ES:0(DI)								
  mixing_amd64.go:40		0x14054d663		8d72ff				LEAL -0x1(DX), SI								
  mixing_amd64.go:40		0x14054d666		4863f6				MOVSXD SI, SI									
  mixing_amd64.go:40		0x14054d669		4883fe05			CMPQ SI, $0x5									
  mixing_amd64.go:40		0x14054d66d		0f83dd010000			JAE 0x14054d850									
  mixing_amd64.go:41		0x14054d673		488b8c2478080000		MOVQ 0x878(SP), CX								
  mixing_amd64.go:41		0x14054d67b		488d71ff			LEAQ -0x1(CX), SI								
  mixing_amd64.go:41		0x14054d67f		90				NOPL										
  mixing_amd64.go:41		0x14054d680		4881fee0010000			CMPQ SI, $0x1e0									
  mixing_amd64.go:41		0x14054d687		0f83b9010000			JAE 0x14054d846									
  cpu.go:43			0x14054d68d		803d116e8d0000			CMPB internal/cpu.X86+69(SB), $0x0						
  mixing_amd64.go:44		0x14054d694		7404				JE 0x14054d69a									
  mixing_amd64.go:44		0x14054d696		31f6				XORL SI, SI									
  mixing_amd64.go:44		0x14054d698		eb6c				JMP 0x14054d706									
  mixing_amd64.go:45		0x14054d69a		4881c460080000			ADDQ $0x860, SP									
  mixing_amd64.go:45		0x14054d6a1		5d				POPQ BP										
  mixing_amd64.go:45		0x14054d6a2		c3				RET										
  mixing_amd64.go:32		0x14054d6a3		488b942478080000		MOVQ 0x878(SP), DX								
  mixing_amd64.go:32		0x14054d6ab		4881fae0010000			CMPQ DX, $0x1e0									
  mixing_amd64.go:32		0x14054d6b2		773a				JA 0x14054d6ee									
  mixing_amd64.go:32		0x14054d6b4		bee0010000			MOVL $.file+403(SB), SI								
  mixing_amd64.go:32		0x14054d6b9		480f4cf2			CMOVL DX, SI									
  mixing_amd64.go:32		0x14054d6bd		488d1436			LEAQ 0(SI)(SI*1), DX								
  mixing_amd64.go:32		0x14054d6c1		4883c310			ADDQ $0x10, BX									
  mixing_amd64.go:32		0x14054d6c5		4889c8				MOVQ CX, AX									
  mixing_amd64.go:32		0x14054d6c8		4889d1				MOVQ DX, CX									
  mixing_amd64.go:32		0x14054d6cb		e85034b4ff			CALL runtime.memmove(SB)							
  mixing_amd64.go:33		0x14054d6d0		48c78424d007000000000000	MOVQ $0x0, 0x7d0(SP)								
  mixing_amd64.go:34		0x14054d6dc		4881c460080000			ADDQ $0x860, SP									
  mixing_amd64.go:34		0x14054d6e3		5d				POPQ BP										
  mixing_amd64.go:34		0x14054d6e4		c3				RET										
  mixing_amd64.go:27		0x14054d6e5		4881c460080000			ADDQ $0x860, SP									
  mixing_amd64.go:27		0x14054d6ec		5d				POPQ BP										
  mixing_amd64.go:27		0x14054d6ed		c3				RET										
  mixing_amd64.go:32		0x14054d6ee		b8e0010000			MOVL $.file+403(SB), AX								
  mixing_amd64.go:32		0x14054d6f3		e84831b4ff			CALL runtime.panicBounds(SB)							
  mixing_amd64.go:59		0x14054d6f8		4c8d4c341c			LEAQ 0x1c(SP)(SI*1), R9								
  mixing_amd64.go:59		0x14054d6fd		62d1fe487f09			VMOVDQU64 Z1, 0(R9)								
  mixing_amd64.go:51		0x14054d703		4889fe				MOVQ DI, SI									
  mixing_amd64.go:51		0x14054d706		488d7e10			LEAQ 0x10(SI), DI								
  mixing_amd64.go:51		0x14054d70a		4839f9				CMPQ CX, DI									
  mixing_amd64.go:51		0x14054d70d		7c73				JL 0x14054d782									
  other_gen_amd64.go:211	0x14054d70f		4531c0				XORL R8, R8									
  other_gen_amd64.go:211	0x14054d712		c4c1796ec8			VMOVD R8, X1									
  mixing_amd64.go:52		0x14054d717		90				NOPL										
  mixing_amd64.go:55		0x14054d718		4c8d0c36			LEAQ 0(SI)(SI*1), R9								
  other_gen_amd64.go:211	0x14054d71c		62f27d4858c9			VPBROADCASTD X1, Z1								
  mixing_amd64.go:53		0x14054d722		4531d2				XORL R10, R10									
  mixing_amd64.go:53		0x14054d725		eb1e				JMP 0x14054d745									
  mixing_amd64.go:55		0x14054d727		4d63da				MOVSXD R10, R11									
  mixing_amd64.go:55		0x14054d72a		4d89cc				MOVQ R9, R12									
  mixing_amd64.go:55		0x14054d72d		4e03a4dcd0070000		ADDQ 0x7d0(SP)(R11*8), R12							
  mixing_amd64.go:56		0x14054d735		62d27d48231424			VPMOVSXWD 0(R12), Z2								
  mixing_amd64.go:57		0x14054d73c		62f17548feca			VPADDD Z2, Z1, Z1								
  mixing_amd64.go:53		0x14054d742		41ffc2				INCL R10									
  mixing_amd64.go:53		0x14054d745		4139d2				CMPL R10, DX									
  mixing_amd64.go:53		0x14054d748		7cdd				JL 0x14054d727									
  mixing_amd64.go:59		0x14054d74a		4881fee0010000			CMPQ SI, $0x1e0									
  mixing_amd64.go:59		0x14054d751		0f87e5000000			JA 0x14054d83c									
  mixing_amd64.go:59		0x14054d757		4c8d8e20feffff			LEAQ 0xfffffe20(SI), R9								
  mixing_amd64.go:59		0x14054d75e		4d89ca				MOVQ R9, R10									
  mixing_amd64.go:59		0x14054d761		49f7d9				NEGQ R9										
  mixing_amd64.go:59		0x14054d764		48c1e602			SHLQ $0x2, SI									
  mixing_amd64.go:59		0x14054d768		49c1fa3f			SARQ $0x3f, R10									
  mixing_amd64.go:59		0x14054d76c		4c21d6				ANDQ R10, SI									
  mixing_amd64.go:59		0x14054d76f		4983f910			CMPQ R9, $0x10									
  mixing_amd64.go:59		0x14054d773		7383				JAE 0x14054d6f8									
  mixing_amd64.go:59		0x14054d775		e9bd000000			JMP 0x14054d837									
  mixing_amd64.go:68		0x14054d77a		448944b41c			MOVL R8, 0x1c(SP)(SI*4)								
  mixing_amd64.go:63		0x14054d77f		48ffc6				INCQ SI										
  mixing_amd64.go:63		0x14054d782		4839f1				CMPQ CX, SI									
  mixing_amd64.go:63		0x14054d785		7e34				JLE 0x14054d7bb									
  mixing_amd64.go:66		0x14054d787		488d3c36			LEAQ 0(SI)(SI*1), DI								
  mixing_amd64.go:65		0x14054d78b		4531c0				XORL R8, R8									
  mixing_amd64.go:65		0x14054d78e		4531c9				XORL R9, R9									
  mixing_amd64.go:65		0x14054d791		eb18				JMP 0x14054d7ab									
  mixing_amd64.go:66		0x14054d793		4d63d1				MOVSXD R9, R10									
  mixing_amd64.go:66		0x14054d796		4989fb				MOVQ DI, R11									
  mixing_amd64.go:66		0x14054d799		4e039cd4d0070000		ADDQ 0x7d0(SP)(R10*8), R11							
  mixing_amd64.go:66		0x14054d7a1		4d0fbf13			MOVSX 0(R11), R10								
  mixing_amd64.go:66		0x14054d7a5		4501d0				ADDL R10, R8									
  mixing_amd64.go:65		0x14054d7a8		41ffc1				INCL R9										
  mixing_amd64.go:65		0x14054d7ab		4139d1				CMPL R9, DX									
  mixing_amd64.go:65		0x14054d7ae		7ce3				JL 0x14054d793									
  mixing_amd64.go:68		0x14054d7b0		4881fee0010000			CMPQ SI, $0x1e0									
  mixing_amd64.go:68		0x14054d7b7		72c1				JB 0x14054d77a									
  mixing_amd64.go:68		0x14054d7b9		eb72				JMP 0x14054d82d									
  mixing_amd64.go:68		0x14054d7bb		31c0				XORL AX, AX									
  mixing_amd64.go:74		0x14054d7bd		eb44				JMP 0x14054d803									
  mixing_amd64.go:74		0x14054d7bf		48898424b0070000		MOVQ AX, 0x7b0(SP)								
  mixing_amd64.go:75		0x14054d7c7		8b44841c			MOVL 0x1c(SP)(AX*4), AX								
  mixing_amd64.go:75		0x14054d7cb		f20f1005cd962400		MOVSD_XMM $f64.40dfffc000000000(SB), X0						
  mixing_amd64.go:75		0x14054d7d3		e828faffff			CALL github.com/gregriff/vogo/cli/internal/audio.softSaturate(SB)		
  mixing_amd64.go:75		0x14054d7d8		488b8c24b0070000		MOVQ 0x7b0(SP), CX								
  mixing_amd64.go:75		0x14054d7e0		488b9c2470080000		MOVQ 0x870(SP), BX								
  mixing_amd64.go:75		0x14054d7e8		6689844bd0120000		MOVW AX, 0x12d0(BX)(CX*2)							
  mixing_amd64.go:74		0x14054d7f0		488d4101			LEAQ 0x1(CX), AX								
  mixing_amd64.go:74		0x14054d7f4		488b8c2478080000		MOVQ 0x878(SP), CX								
  mixing_amd64.go:79		0x14054d7fc		8b94249c070000			MOVL 0x79c(SP), DX								
  mixing_amd64.go:74		0x14054d803		4839c1				CMPQ CX, AX									
  mixing_amd64.go:74		0x14054d806		7fb7				JG 0x14054d7bf									
  mixing_amd64.go:79		0x14054d808		31c0				XORL AX, AX									
  mixing_amd64.go:79		0x14054d80a		eb14				JMP 0x14054d820									
  mixing_amd64.go:80		0x14054d80c		4863c8				MOVSXD AX, CX									
  mixing_amd64.go:80		0x14054d80f		48c784ccd007000000000000	MOVQ $0x0, 0x7d0(SP)(CX*8)							
  mixing_amd64.go:79		0x14054d81b		ffc0				INCL AX										
  mixing_amd64.go:79		0x14054d81d		0f1f00				NOPL 0(AX)									
  mixing_amd64.go:79		0x14054d820		39d0				CMPL AX, DX									
  mixing_amd64.go:79		0x14054d822		7ce8				JL 0x14054d80c									
  mixing_amd64.go:82		0x14054d824		4881c460080000			ADDQ $0x860, SP									
  mixing_amd64.go:82		0x14054d82b		5d				POPQ BP										
  mixing_amd64.go:82		0x14054d82c		c3				RET										
  mixing_amd64.go:68		0x14054d82d		b8e0010000			MOVL $.file+403(SB), AX								
  mixing_amd64.go:68		0x14054d832		e80930b4ff			CALL runtime.panicBounds(SB)							
  mixing_amd64.go:59		0x14054d837		e80430b4ff			CALL runtime.panicBounds(SB)							
  mixing_amd64.go:59		0x14054d83c		b8e0010000			MOVL $.file+403(SB), AX								
  mixing_amd64.go:59		0x14054d841		e8fa2fb4ff			CALL runtime.panicBounds(SB)							
  mixing_amd64.go:41		0x14054d846		b8e0010000			MOVL $.file+403(SB), AX								
  mixing_amd64.go:41		0x14054d84b		e8f02fb4ff			CALL runtime.panicBounds(SB)							
  mixing_amd64.go:40		0x14054d850		e8eb2fb4ff			CALL runtime.panicBounds(SB)							
  ringbuffer.go:79		0x14054d855		e806eeafff			CALL runtime.panicdivide(SB)							
  ringbuffer.go:76		0x14054d85a		e8e12fb4ff			CALL runtime.panicBounds(SB)							
  ringbuffer.go:76		0x14054d85f		b8e0010000			MOVL $.file+403(SB), AX								
  ringbuffer.go:76		0x14054d864		e8d72fb4ff			CALL runtime.panicBounds(SB)							
  ringbuffer.go:75		0x14054d869		e8d22fb4ff			CALL runtime.panicBounds(SB)							
  ringbuffer.go:73		0x14054d86e		e8cd2fb4ff			CALL runtime.panicBounds(SB)							
  ringbuffer.go:73		0x14054d873		e8c82fb4ff			CALL runtime.panicBounds(SB)							
  mixing_amd64.go:16		0x14054d878		e8c32fb4ff			CALL runtime.panicBounds(SB)							
  mixing_amd64.go:16		0x14054d87d		90				NOPL										
  mixing_amd64.go:9		0x14054d87e		4889442408			MOVQ AX, 0x8(SP)								
  mixing_amd64.go:9		0x14054d883		48895c2410			MOVQ BX, 0x10(SP)								
  mixing_amd64.go:9		0x14054d888		e85311b4ff			CALL runtime.morestack_noctxt.abi0(SB)						
  mixing_amd64.go:9		0x14054d88d		488b442408			MOVQ 0x8(SP), AX								
  mixing_amd64.go:9		0x14054d892		488b5c2410			MOVQ 0x10(SP), BX								
  mixing_amd64.go:9		0x14054d897		e924faffff			JMP github.com/gregriff/vogo/cli/internal/audio.(*streams).mixAVX512(SB)	
  :-1				0x14054d89c		cc				INT $0x3									
  :-1				0x14054d89d		cc				INT $0x3									
  :-1				0x14054d89e		cc				INT $0x3									
  :-1				0x14054d89f		cc				INT $0x3									

TEXT github.com/gregriff/vogo/cli/internal/audio.(*streams).mixAVX512-fm(SB) <autogenerated>
  <autogenerated>:1	0x14054fbc0		493b6610		CMPQ SP, 0x10(R14)								
  <autogenerated>:1	0x14054fbc4		761d			JBE 0x14054fbe3									
  <autogenerated>:1	0x14054fbc6		55			PUSHQ BP									
  <autogenerated>:1	0x14054fbc7		4889e5			MOVQ SP, BP									
  <autogenerated>:1	0x14054fbca		4883ec10		SUBQ $0x10, SP									
  <autogenerated>:1	0x14054fbce		488b4a08		MOVQ 0x8(DX), CX								
  <autogenerated>:1	0x14054fbd2		4889c3			MOVQ AX, BX									
  <autogenerated>:1	0x14054fbd5		4889c8			MOVQ CX, AX									
  <autogenerated>:1	0x14054fbd8		e8e3d6ffff		CALL github.com/gregriff/vogo/cli/internal/audio.(*streams).mixAVX512(SB)	
  <autogenerated>:1	0x14054fbdd		4883c410		ADDQ $0x10, SP									
  <autogenerated>:1	0x14054fbe1		5d			POPQ BP										
  <autogenerated>:1	0x14054fbe2		c3			RET										
  <autogenerated>:1	0x14054fbe3		4889442408		MOVQ AX, 0x8(SP)								
  <autogenerated>:1	0x14054fbe8		e873edb3ff		CALL runtime.morestack.abi0(SB)							
  <autogenerated>:1	0x14054fbed		488b442408		MOVQ 0x8(SP), AX								
  <autogenerated>:1	0x14054fbf2		ebcc			JMP github.com/gregriff/vogo/cli/internal/audio.(*streams).mixAVX512-fm(SB)	
  :-1			0x14054fbf4		cc			INT $0x3									
  :-1			0x14054fbf5		cc			INT $0x3									
  :-1			0x14054fbf6		cc			INT $0x3									
  :-1			0x14054fbf7		cc			INT $0x3									
  :-1			0x14054fbf8		cc			INT $0x3									
  :-1			0x14054fbf9		cc			INT $0x3									
  :-1			0x14054fbfa		cc			INT $0x3									
  :-1			0x14054fbfb		cc			INT $0x3									
  :-1			0x14054fbfc		cc			INT $0x3									
  :-1			0x14054fbfd		cc			INT $0x3									
  :-1			0x14054fbfe		cc			INT $0x3									
  :-1			0x14054fbff		cc			INT $0x3									
