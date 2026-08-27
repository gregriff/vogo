TEXT github.com/gregriff/vogo/cli/internal/audio.(*streams).mixAVX512(SB) C:/Users/gregg/code/vogo/cli/internal/audio/mixing_amd64.go
  mixing_amd64.go:9             0x14054d3c0             4c8d642498              LEAQ -0x68(SP), R12
  mixing_amd64.go:9             0x14054d3c5             4d3b6610                CMPQ R12, 0x10(R14)
  mixing_amd64.go:9             0x14054d3c9             0f8601060000            JBE 0x14054d9d0
  mixing_amd64.go:9             0x14054d3cf             55                      PUSHQ BP
  mixing_amd64.go:9             0x14054d3d0             4889e5                  MOVQ SP, BP
  mixing_amd64.go:9             0x14054d3d3             4881ece0000000          SUBQ $0xe0, SP
  mixing_amd64.go:13            0x14054d3da             48898424f0000000        MOVQ AX, 0xf0(SP)
  mixing_amd64.go:13            0x14054d3e2             48899c24f8000000        MOVQ BX, 0xf8(SP)
  mixing_amd64.go:11            0x14054d3ea             488d542450              LEAQ 0x50(SP), DX
  mixing_amd64.go:11            0x14054d3ef             440f113a                MOVUPS X15, 0(DX)
  mixing_amd64.go:11            0x14054d3f3             440f117a10              MOVUPS X15, 0x10(DX)
  mixing_amd64.go:11            0x14054d3f8             440f117a18              MOVUPS X15, 0x18(DX)
  mixing_amd64.go:13            0x14054d3fd             488b5808                MOVQ 0x8(AX), BX
  mixing_amd64.go:13            0x14054d401             488d4c2478              LEAQ 0x78(SP), CX
  mixing_amd64.go:13            0x14054d406             440f1139                MOVUPS X15, 0(CX)
  mixing_amd64.go:13            0x14054d40a             440f117910              MOVUPS X15, 0x10(CX)
  mixing_amd64.go:13            0x14054d40f             440f117920              MOVUPS X15, 0x20(CX)
  mixing_amd64.go:13            0x14054d414             440f117930              MOVUPS X15, 0x30(CX)
  mixing_amd64.go:13            0x14054d419             440f117940              MOVUPS X15, 0x40(CX)
  mixing_amd64.go:13            0x14054d41e             440f117950              MOVUPS X15, 0x50(CX)
  mixing_amd64.go:13            0x14054d423             488d05be887600          LEAQ type:*+452400(SB), AX
  mixing_amd64.go:13            0x14054d42a             e87154adff              CALL runtime.mapIterStart(SB)
  mixing_amd64.go:13            0x14054d42f             31c0                    XORL AX, AX
  mixing_amd64.go:13            0x14054d431             eb17                    JMP 0x14054d44a
  mixing_amd64.go:13            0x14054d433             4889442428              MOVQ AX, 0x28(SP)
  mixing_amd64.go:13            0x14054d438             488d442478              LEAQ 0x78(SP), AX
  mixing_amd64.go:13            0x14054d43d             0f1f00                  NOPL 0(AX)
  mixing_amd64.go:13            0x14054d440             e8bb54adff              CALL runtime.mapIterNext(SB)
  mixing_amd64.go:13            0x14054d445             488b442428              MOVQ 0x28(SP), AX
  mixing_amd64.go:13            0x14054d44a             4889442428              MOVQ AX, 0x28(SP)
  mixing_amd64.go:13            0x14054d44f             48837c247800            CMPQ 0x78(SP), $0x0
  mixing_amd64.go:13            0x14054d455             0f8475020000            JE 0x14054d6d0
  mixing_amd64.go:13            0x14054d45b             488b942480000000        MOVQ 0x80(SP), DX
  mixing_amd64.go:13            0x14054d463             488b12                  MOVQ 0(DX), DX
  ringbuffer.go:85              0x14054d466             488b7228                MOVQ 0x28(DX), SI
  mixing_amd64.go:14            0x14054d46a             488bbc24f8000000        MOVQ 0xf8(SP), DI
  mixing_amd64.go:14            0x14054d472             4839f7                  CMPQ DI, SI
  mixing_amd64.go:14            0x14054d475             7fbc                    JG 0x14054d433
  mixing_amd64.go:14            0x14054d477             660f1f840000000000      NOPW 0(AX)(AX*1)
  mixing_amd64.go:16            0x14054d480             4883f805                CMPQ AX, $0x5
  mixing_amd64.go:16            0x14054d484             0f8340050000            JAE 0x14054d9ca
  mixing_amd64.go:16            0x14054d48a             4c69c0c0030000          IMULQ $0x3c0, AX, R8
  mixing_amd64.go:16            0x14054d491             4c8b8c24f0000000        MOVQ 0xf0(SP), R9
  mixing_amd64.go:16            0x14054d499             4f8d1401                LEAQ 0(R9)(R8*1), R10
  mixing_amd64.go:16            0x14054d49d             4d8d5210                LEAQ 0x10(R10), R10
  ringbuffer.go:66              0x14054d4a1             4881fee0010000          CMPQ SI, $0x1e0
  ringbuffer.go:67              0x14054d4a8             41bbe0010000            MOVL $.file+403(SB), R11
  ringbuffer.go:67              0x14054d4ae             490f4ff3                CMOVG R11, SI
  ringbuffer.go:67              0x14054d4b2             4885f6                  TESTQ SI, SI
  ringbuffer.go:66              0x14054d4b5             7508                    JNE 0x14054d4bf
  mixing_amd64.go:19            0x14054d4b7             4989c2                  MOVQ AX, R10
  mixing_amd64.go:16            0x14054d4ba             e9fb010000              JMP 0x14054d6ba
  mixing_amd64.go:13            0x14054d4bf             4889542448              MOVQ DX, 0x48(SP)
  ringbuffer.go:67              0x14054d4c4             4889742418              MOVQ SI, 0x18(SP)
  mixing_amd64.go:16            0x14054d4c9             4c89442440              MOVQ R8, 0x40(SP)
  ringbuffer.go:71              0x14054d4ce             4c8b6230                MOVQ 0x30(DX), R12
  ringbuffer.go:71              0x14054d4d2             4c8b6a18                MOVQ 0x18(DX), R13
  ringbuffer.go:71              0x14054d4d6             4d29ec                  SUBQ R13, R12
  ringbuffer.go:71              0x14054d4d9             0f1f8000000000          NOPL 0(AX)
  ringbuffer.go:72              0x14054d4e0             4c39e6                  CMPQ SI, R12
  ringbuffer.go:72              0x14054d4e3             0f8f7f000000            JG 0x14054d568
  ringbuffer.go:73              0x14054d4e9             4c8b6210                MOVQ 0x10(DX), R12
  ringbuffer.go:73              0x14054d4ed             4d8d7c3500              LEAQ 0(R13)(SI*1), R15
  ringbuffer.go:73              0x14054d4f2             4d39fc                  CMPQ R12, R15
  ringbuffer.go:73              0x14054d4f5             0f82ca040000            JB 0x14054d9c5
  ringbuffer.go:73              0x14054d4fb             0f1f440000              NOPL 0(AX)(AX*1)
  ringbuffer.go:73              0x14054d500             4d39fd                  CMPQ R13, R15
  ringbuffer.go:73              0x14054d503             0f87b5040000            JA 0x14054d9be
  ringbuffer.go:73              0x14054d509             4c8b3a                  MOVQ 0(DX), R15
  ringbuffer.go:73              0x14054d50c             4b8d4c2d00              LEAQ 0(R13)(R13*1), CX
  ringbuffer.go:73              0x14054d511             4d29e5                  SUBQ R12, R13
  ringbuffer.go:73              0x14054d514             49c1fd3f                SARQ $0x3f, R13
  ringbuffer.go:73              0x14054d518             4c21e9                  ANDQ R13, CX
  ringbuffer.go:73              0x14054d51b             4a8d1c39                LEAQ 0(CX)(R15*1), BX
  ringbuffer.go:73              0x14054d51f             4881fee0010000          CMPQ SI, $0x1e0
  ringbuffer.go:73              0x14054d526             4c0f4cde                CMOVL SI, R11
  ringbuffer.go:73              0x14054d52a             4939da                  CMPQ R10, BX
  ringbuffer.go:73              0x14054d52d             0f844b010000            JE 0x14054d67e
  ringbuffer.go:73              0x14054d533             4b8d0c1b                LEAQ 0(R11)(R11*1), CX
  ringbuffer.go:73              0x14054d537             4c89d0                  MOVQ R10, AX
  ringbuffer.go:73              0x14054d53a             e8e135b4ff              CALL runtime.memmove(SB)
  mixing_amd64.go:19            0x14054d53f             488b442428              MOVQ 0x28(SP), AX
  ringbuffer.go:79              0x14054d544             488b542448              MOVQ 0x48(SP), DX
  ringbuffer.go:79              0x14054d549             488b742418              MOVQ 0x18(SP), SI
  mixing_amd64.go:14            0x14054d54e             488bbc24f8000000        MOVQ 0xf8(SP), DI
  mixing_amd64.go:19            0x14054d556             4c8b442440              MOVQ 0x40(SP), R8
  mixing_amd64.go:19            0x14054d55b             4c8b8c24f0000000        MOVQ 0xf0(SP), R9
  ringbuffer.go:73              0x14054d563             e916010000              JMP 0x14054d67e
  ringbuffer.go:75              0x14054d568             4c8b7a08                MOVQ 0x8(DX), R15
  ringbuffer.go:75              0x14054d56c             4d39ef                  CMPQ R15, R13
  ringbuffer.go:75              0x14054d56f             0f8244040000            JB 0x14054d9b9
  ringbuffer.go:71              0x14054d575             4c896c2438              MOVQ R13, 0x38(SP)
  ringbuffer.go:75              0x14054d57a             488b0a                  MOVQ 0(DX), CX
  ringbuffer.go:75              0x14054d57d             488b5a10                MOVQ 0x10(DX), BX
  ringbuffer.go:75              0x14054d581             4d29ef                  SUBQ R13, R15
  ringbuffer.go:75              0x14054d584             4929dd                  SUBQ BX, R13
  mixing_amd64.go:13            0x14054d587             4889fb                  MOVQ DI, BX
  ringbuffer.go:75              0x14054d58a             488b7c2438              MOVQ 0x38(SP), DI
  ringbuffer.go:75              0x14054d58f             4801ff                  ADDQ DI, DI
  ringbuffer.go:75              0x14054d592             49c1fd3f                SARQ $0x3f, R13
  ringbuffer.go:75              0x14054d596             4c21ef                  ANDQ R13, DI
  ringbuffer.go:75              0x14054d599             4801cf                  ADDQ CX, DI
  ringbuffer.go:75              0x14054d59c             4981ffe0010000          CMPQ R15, $0x1e0
  ringbuffer.go:75              0x14054d5a3             4d0f4cdf                CMOVL R15, R11
  ringbuffer.go:75              0x14054d5a7             4939fa                  CMPQ R10, DI
  ringbuffer.go:75              0x14054d5aa             7454                    JE 0x14054d600
  mixing_amd64.go:16            0x14054d5ac             4c899424d8000000        MOVQ R10, 0xd8(SP)
  ringbuffer.go:71              0x14054d5b4             4c89642420              MOVQ R12, 0x20(SP)
  ringbuffer.go:75              0x14054d5b9             4b8d0c1b                LEAQ 0(R11)(R11*1), CX
  ringbuffer.go:75              0x14054d5bd             4c89d0                  MOVQ R10, AX
  ringbuffer.go:75              0x14054d5c0             4889fb                  MOVQ DI, BX
  ringbuffer.go:75              0x14054d5c3             e85835b4ff              CALL runtime.memmove(SB)
  mixing_amd64.go:19            0x14054d5c8             488b442428              MOVQ 0x28(SP), AX
  ringbuffer.go:76              0x14054d5cd             488b542448              MOVQ 0x48(SP), DX
  mixing_amd64.go:14            0x14054d5d2             488b9c24f8000000        MOVQ 0xf8(SP), BX
  ringbuffer.go:76              0x14054d5da             488b742418              MOVQ 0x18(SP), SI
  mixing_amd64.go:19            0x14054d5df             4c8b442440              MOVQ 0x40(SP), R8
  mixing_amd64.go:19            0x14054d5e4             4c8b8c24f0000000        MOVQ 0xf0(SP), R9
  ringbuffer.go:76              0x14054d5ec             4c8b9424d8000000        MOVQ 0xd8(SP), R10
  ringbuffer.go:76              0x14054d5f4             4c8b642420              MOVQ 0x20(SP), R12
  ringbuffer.go:76              0x14054d5f9             0f1f8000000000          NOPL 0(AX)
  ringbuffer.go:76              0x14054d600             4981fce0010000          CMPQ R12, $0x1e0
  ringbuffer.go:76              0x14054d607             0f87a2030000            JA 0x14054d9af
  ringbuffer.go:76              0x14054d60d             488b7a10                MOVQ 0x10(DX), DI
  ringbuffer.go:76              0x14054d611             4989f3                  MOVQ SI, R11
  ringbuffer.go:76              0x14054d614             4c29e6                  SUBQ R12, SI
  ringbuffer.go:76              0x14054d617             4f8d1462                LEAQ 0(R10)(R12*2), R10
  ringbuffer.go:76              0x14054d61b             0f1f440000              NOPL 0(AX)(AX*1)
  ringbuffer.go:76              0x14054d620             4839f7                  CMPQ DI, SI
  ringbuffer.go:76              0x14054d623             0f8281030000            JB 0x14054d9aa
  ringbuffer.go:76              0x14054d629             498dbc2420feffff        LEAQ 0xfffffe20(R12), DI
  ringbuffer.go:76              0x14054d631             48f7df                  NEGQ DI
  ringbuffer.go:76              0x14054d634             4c8b22                  MOVQ 0(DX), R12
  ringbuffer.go:76              0x14054d637             4839f7                  CMPQ DI, SI
  ringbuffer.go:76              0x14054d63a             480f4ffe                CMOVG SI, DI
  ringbuffer.go:76              0x14054d63e             6690                    NOPW
  ringbuffer.go:76              0x14054d640             4d39d4                  CMPQ R12, R10
  ringbuffer.go:76              0x14054d643             7433                    JE 0x14054d678
  ringbuffer.go:76              0x14054d645             488d0c3f                LEAQ 0(DI)(DI*1), CX
  ringbuffer.go:76              0x14054d649             4c89d0                  MOVQ R10, AX
  ringbuffer.go:76              0x14054d64c             4c89e3                  MOVQ R12, BX
  ringbuffer.go:76              0x14054d64f             e8cc34b4ff              CALL runtime.memmove(SB)
  mixing_amd64.go:19            0x14054d654             488b442428              MOVQ 0x28(SP), AX
  ringbuffer.go:79              0x14054d659             488b542448              MOVQ 0x48(SP), DX
  mixing_amd64.go:14            0x14054d65e             488b9c24f8000000        MOVQ 0xf8(SP), BX
  mixing_amd64.go:19            0x14054d666             4c8b442440              MOVQ 0x40(SP), R8
  mixing_amd64.go:19            0x14054d66b             4c8b8c24f0000000        MOVQ 0xf0(SP), R9
  ringbuffer.go:79              0x14054d673             4c8b5c2418              MOVQ 0x18(SP), R11
  ringbuffer.go:79              0x14054d678             4c89de                  MOVQ R11, SI
  mixing_amd64.go:14            0x14054d67b             4889df                  MOVQ BX, DI
  ringbuffer.go:79              0x14054d67e             488b4a18                MOVQ 0x18(DX), CX
  ringbuffer.go:79              0x14054d682             488b5a30                MOVQ 0x30(DX), BX
  ringbuffer.go:79              0x14054d686             4801f1                  ADDQ SI, CX
  ringbuffer.go:79              0x14054d689             4885db                  TESTQ BX, BX
  ringbuffer.go:79              0x14054d68c             0f8413030000            JE 0x14054d9a5
  mixing_amd64.go:13            0x14054d692             4989c2                  MOVQ AX, R10
  ringbuffer.go:79              0x14054d695             4889c8                  MOVQ CX, AX
  mixing_amd64.go:13            0x14054d698             4889d1                  MOVQ DX, CX
  mixing_amd64.go:13            0x14054d69b             0f1f440000              NOPL 0(AX)(AX*1)
  ringbuffer.go:79              0x14054d6a0             4883fbff                CMPQ BX, $-0x1
  ringbuffer.go:79              0x14054d6a4             7507                    JNE 0x14054d6ad
  ringbuffer.go:79              0x14054d6a6             48f7d8                  NEGQ AX
  ringbuffer.go:79              0x14054d6a9             31d2                    XORL DX, DX
  ringbuffer.go:79              0x14054d6ab             eb05                    JMP 0x14054d6b2
  ringbuffer.go:79              0x14054d6ad             4899                    CQO
  ringbuffer.go:79              0x14054d6af             48f7fb                  IDIVQ BX
  ringbuffer.go:79              0x14054d6b2             48895118                MOVQ DX, 0x18(CX)
  ringbuffer.go:80              0x14054d6b6             48297128                SUBQ SI, 0x28(CX)
  mixing_amd64.go:19            0x14054d6ba             4b8d0c01                LEAQ 0(R9)(R8*1), CX
  mixing_amd64.go:19            0x14054d6be             488d4910                LEAQ 0x10(CX), CX
  mixing_amd64.go:19            0x14054d6c2             4a894cd450              MOVQ CX, 0x50(SP)(R10*8)
  mixing_amd64.go:20            0x14054d6c7             498d4201                LEAQ 0x1(R10), AX
  mixing_amd64.go:20            0x14054d6cb             e963fdffff              JMP 0x14054d433
  mixing_amd64.go:26            0x14054d6d0             488b9c24f0000000        MOVQ 0xf0(SP), BX
  mixing_amd64.go:26            0x14054d6d8             488d93d0120000          LEAQ 0x12d0(BX), DX
  mixing_amd64.go:26            0x14054d6df             4889d1                  MOVQ DX, CX
  mixing_amd64.go:26            0x14054d6e2             be0f000000              MOVL $__major_subsystem_version__+5(SB), SI
  mixing_amd64.go:26            0x14054d6e7             440f113a                MOVUPS X15, 0(DX)
  mixing_amd64.go:26            0x14054d6eb             440f117a10              MOVUPS X15, 0x10(DX)
  mixing_amd64.go:26            0x14054d6f0             440f117a20              MOVUPS X15, 0x20(DX)
  mixing_amd64.go:26            0x14054d6f5             440f117a30              MOVUPS X15, 0x30(DX)
  mixing_amd64.go:26            0x14054d6fa             4883c240                ADDQ $0x40, DX
  mixing_amd64.go:26            0x14054d6fe             6690                    NOPW
  mixing_amd64.go:26            0x14054d700             ffce                    DECL SI
  mixing_amd64.go:26            0x14054d702             75e3                    JNE 0x14054d6e7
  mixing_amd64.go:27            0x14054d704             4885c0                  TESTQ AX, AX
  mixing_amd64.go:27            0x14054d707             0f848a000000            JE 0x14054d797
  mixing_amd64.go:27            0x14054d70d             488b5308                MOVQ 0x8(BX), DX
  mixing_amd64.go:27            0x14054d711             4885d2                  TESTQ DX, DX
  mixing_amd64.go:27            0x14054d714             7405                    JE 0x14054d71b
  mixing_amd64.go:27            0x14054d716             488b12                  MOVQ 0(DX), DX
  mixing_amd64.go:27            0x14054d719             eb05                    JMP 0x14054d720
  mixing_amd64.go:27            0x14054d71b             31d2                    XORL DX, DX
  mixing_amd64.go:27            0x14054d71d             0f1f00                  NOPL 0(AX)
  mixing_amd64.go:27            0x14054d720             4885d2                  TESTQ DX, DX
  mixing_amd64.go:27            0x14054d723             7472                    JE 0x14054d797
  mixing_amd64.go:32            0x14054d725             4883f801                CMPQ AX, $0x1
  mixing_amd64.go:32            0x14054d729             742b                    JE 0x14054d756
  mixing_amd64.go:41            0x14054d72b             488d48ff                LEAQ -0x1(AX), CX
  mixing_amd64.go:41            0x14054d72f             4883f905                CMPQ CX, $0x5
  mixing_amd64.go:41            0x14054d733             0f8366020000            JAE 0x14054d99f
  mixing_amd64.go:42            0x14054d739             488b8c24f8000000        MOVQ 0xf8(SP), CX
  mixing_amd64.go:42            0x14054d741             488d51ff                LEAQ -0x1(CX), DX
  mixing_amd64.go:42            0x14054d745             4881fae0010000          CMPQ DX, $0x1e0
  mixing_amd64.go:42            0x14054d74c             0f8343020000            JAE 0x14054d995
  mixing_amd64.go:48            0x14054d752             31d2                    XORL DX, DX
  mixing_amd64.go:48            0x14054d754             eb69                    JMP 0x14054d7bf
  mixing_amd64.go:33            0x14054d756             488b9424f8000000        MOVQ 0xf8(SP), DX
  mixing_amd64.go:33            0x14054d75e             6690                    NOPW
  mixing_amd64.go:33            0x14054d760             4881fae0010000          CMPQ DX, $0x1e0
  mixing_amd64.go:33            0x14054d767             7738                    JA 0x14054d7a1
  mixing_amd64.go:33            0x14054d769             bee0010000              MOVL $.file+403(SB), SI
  mixing_amd64.go:33            0x14054d76e             480f4cf2                CMOVL DX, SI
  mixing_amd64.go:33            0x14054d772             488d1436                LEAQ 0(SI)(SI*1), DX
  mixing_amd64.go:33            0x14054d776             4883c310                ADDQ $0x10, BX
  mixing_amd64.go:33            0x14054d77a             4889c8                  MOVQ CX, AX
  mixing_amd64.go:33            0x14054d77d             4889d1                  MOVQ DX, CX
  mixing_amd64.go:33            0x14054d780             e89b33b4ff              CALL runtime.memmove(SB)
  mixing_amd64.go:34            0x14054d785             48c744245000000000      MOVQ $0x0, 0x50(SP)
  mixing_amd64.go:35            0x14054d78e             4881c4e0000000          ADDQ $0xe0, SP
  mixing_amd64.go:35            0x14054d795             5d                      POPQ BP
  mixing_amd64.go:35            0x14054d796             c3                      RET
  mixing_amd64.go:28            0x14054d797             4881c4e0000000          ADDQ $0xe0, SP
  mixing_amd64.go:28            0x14054d79e             5d                      POPQ BP
  mixing_amd64.go:28            0x14054d79f             90                      NOPL
  mixing_amd64.go:28            0x14054d7a0             c3                      RET
  mixing_amd64.go:33            0x14054d7a1             b8e0010000              MOVL $.file+403(SB), AX
  mixing_amd64.go:33            0x14054d7a6             e89530b4ff              CALL runtime.panicBounds(SB)
  mixing_amd64.go:78            0x14054d7ab             62f17e485bc9            VCVTTPS2DQ Z1, Z1
  mixing_amd64.go:79            0x14054d7b1             62f27e4823c9            VPMOVSDW Z1, Y1
  mixing_amd64.go:82            0x14054d7b7             c4c17e7f08              VMOVDQU Y1, 0(R8)
  mixing_amd64.go:48            0x14054d7bc             4889f2                  MOVQ SI, DX
  mixing_amd64.go:48            0x14054d7bf             488d7210                LEAQ 0x10(DX), SI
  mixing_amd64.go:48            0x14054d7c3             4839f1                  CMPQ CX, SI
  mixing_amd64.go:48            0x14054d7c6             0f8c44010000            JL 0x14054d910
  other_gen_amd64.go:211        0x14054d7cc             31ff                    XORL DI, DI
  other_gen_amd64.go:211        0x14054d7ce             c5f96ecf                VMOVD DI, X1
  mixing_amd64.go:49            0x14054d7d2             90                      NOPL
  mixing_amd64.go:52            0x14054d7d3             4c8d0412                LEAQ 0(DX)(DX*1), R8
  other_gen_amd64.go:211        0x14054d7d7             62f27d4858c9            VPBROADCASTD X1, Z1
  mixing_amd64.go:50            0x14054d7dd             4531c9                  XORL R9, R9
  mixing_amd64.go:50            0x14054d7e0             eb17                    JMP 0x14054d7f9
  mixing_amd64.go:52            0x14054d7e2             4d89c2                  MOVQ R8, R10
  mixing_amd64.go:52            0x14054d7e5             4e0354cc50              ADDQ 0x50(SP)(R9*8), R10
  mixing_amd64.go:53            0x14054d7ea             62d27d482312            VPMOVSXWD 0(R10), Z2
  mixing_amd64.go:54            0x14054d7f0             62f17548feca            VPADDD Z2, Z1, Z1
  mixing_amd64.go:50            0x14054d7f6             49ffc1                  INCQ R9
  mixing_amd64.go:50            0x14054d7f9             4939c1                  CMPQ R9, AX
  mixing_amd64.go:50            0x14054d7fc             7ce4                    JL 0x14054d7e2
  mixing_amd64.go:62            0x14054d7fe             62f17c485bc9            VCVTDQ2PS Z1, Z1
  other_gen_amd64.go:265        0x14054d804             c5fa101500932400        VMOVSS __size_of_stack_reserve__+299776(SB), X2
  other_gen_amd64.go:265        0x14054d80c             62f27d4818d2            VBROADCASTSS X2, Z2
  mixing_amd64.go:72            0x14054d812             62f174485eca            VDIVPS Z2, Z1, Z1
  other_gen_amd64.go:265        0x14054d818             c5fa101de0922400        VMOVSS __size_of_stack_reserve__+299744(SB), X3
  other_gen_amd64.go:265        0x14054d820             62f27d4818db            VBROADCASTSS X3, Z3
  other_gen_amd64.go:265        0x14054d826             c5fa1025e6922400        VMOVSS __size_of_stack_reserve__+299750(SB), X4
  other_gen_amd64.go:265        0x14054d82e             62f27d4818e4            VBROADCASTSS X4, Z4
  mixing_amd64.go:216           0x14054d834             62f15c485fc9            VMAXPS Z1, Z4, Z1
  mixing_amd64.go:217           0x14054d83a             62f164485dc9            VMINPS Z1, Z3, Z1
  mixing_amd64.go:219           0x14054d840             62f1744859d9            VMULPS Z1, Z1, Z3
  other_gen_amd64.go:265        0x14054d846             c5fa1025aa922400        VMOVSS __size_of_stack_reserve__+299690(SB), X4
  other_gen_amd64.go:265        0x14054d84e             62f27d4818e4            VBROADCASTSS X4, Z4
  other_gen_amd64.go:265        0x14054d854             c5fa102db4922400        VMOVSS __size_of_stack_reserve__+299700(SB), X5
  other_gen_amd64.go:265        0x14054d85c             62f27d4818ed            VBROADCASTSS X5, Z5
  other_gen_amd64.go:265        0x14054d862             c5fa10359e922400        VMOVSS __size_of_stack_reserve__+299678(SB), X6
  other_gen_amd64.go:265        0x14054d86a             62f27d4818f6            VBROADCASTSS X6, Z6
  mixing_amd64.go:222           0x14054d870             62f1644858fe            VADDPS Z6, Z3, Z7
  mixing_amd64.go:223           0x14054d876             62f1444859c9            VMULPS Z1, Z7, Z1
  other_gen_amd64.go:265        0x14054d87c             c5fa103d80922400        VMOVSS __size_of_stack_reserve__+299648(SB), X7
  other_gen_amd64.go:265        0x14054d884             62f27d4818ff            VBROADCASTSS X7, Z7
  mixing_amd64.go:226           0x14054d88a             62f24548a8de            VFMADD213PS Z6, Z7, Z3
  mixing_amd64.go:234           0x14054d890             62f174485ecb            VDIVPS Z3, Z1, Z1
  mixing_amd64.go:237           0x14054d896             62f174485fcd            VMAXPS Z5, Z1, Z1
  mixing_amd64.go:238           0x14054d89c             62f15c485dc9            VMINPS Z1, Z4, Z1
  mixing_amd64.go:67            0x14054d8a2             90                      NOPL
  mixing_amd64.go:73            0x14054d8a3             90                      NOPL
  mixing_amd64.go:76            0x14054d8a4             62f16c4859c9            VMULPS Z1, Z2, Z1
  mixing_amd64.go:203           0x14054d8aa             90                      NOPL
  mixing_amd64.go:204           0x14054d8ab             90                      NOPL
  mixing_amd64.go:205           0x14054d8ac             90                      NOPL
  mixing_amd64.go:206           0x14054d8ad             90                      NOPL
  mixing_amd64.go:209           0x14054d8ae             90                      NOPL
  mixing_amd64.go:210           0x14054d8af             90                      NOPL
  mixing_amd64.go:82            0x14054d8b0             4881fae0010000          CMPQ DX, $0x1e0
  mixing_amd64.go:82            0x14054d8b7             0f87ce000000            JA 0x14054d98b
  mixing_amd64.go:82            0x14054d8bd             4c8d0412                LEAQ 0(DX)(DX*1), R8
  mixing_amd64.go:82            0x14054d8c1             4881c220feffff          ADDQ $-0x1e0, DX
  mixing_amd64.go:82            0x14054d8c8             4989d1                  MOVQ DX, R9
  mixing_amd64.go:82            0x14054d8cb             48f7da                  NEGQ DX
  mixing_amd64.go:82            0x14054d8ce             49c1f93f                SARQ $0x3f, R9
  mixing_amd64.go:82            0x14054d8d2             4d21c8                  ANDQ R9, R8
  mixing_amd64.go:82            0x14054d8d5             4e8d0403                LEAQ 0(BX)(R8*1), R8
  mixing_amd64.go:82            0x14054d8d9             4d8d80d0120000          LEAQ 0x12d0(R8), R8
  mixing_amd64.go:82            0x14054d8e0             4883fa10                CMPQ DX, $0x10
  mixing_amd64.go:82            0x14054d8e4             0f83c1feffff            JAE 0x14054d7ab
  mixing_amd64.go:82            0x14054d8ea             e997000000              JMP 0x14054d986
  mixing_amd64.go:91            0x14054d8ef             488b9c24f0000000        MOVQ 0xf0(SP), BX
  mixing_amd64.go:91            0x14054d8f7             6689844bd0120000        MOVW AX, 0x12d0(BX)(CX*2)
  mixing_amd64.go:86            0x14054d8ff             488d5101                LEAQ 0x1(CX), DX
  mixing_amd64.go:88            0x14054d903             488b442428              MOVQ 0x28(SP), AX
  mixing_amd64.go:86            0x14054d908             488b8c24f8000000        MOVQ 0xf8(SP), CX
  mixing_amd64.go:86            0x14054d910             4839d1                  CMPQ CX, DX
  mixing_amd64.go:86            0x14054d913             7e49                    JLE 0x14054d95e
  mixing_amd64.go:89            0x14054d915             488d3412                LEAQ 0(DX)(DX*1), SI
  mixing_amd64.go:88            0x14054d919             31ff                    XORL DI, DI
  mixing_amd64.go:88            0x14054d91b             4531c0                  XORL R8, R8
  mixing_amd64.go:88            0x14054d91e             6690                    NOPW
  mixing_amd64.go:88            0x14054d920             eb12                    JMP 0x14054d934
  mixing_amd64.go:89            0x14054d922             4989f1                  MOVQ SI, R9
  mixing_amd64.go:89            0x14054d925             4c034cfc50              ADDQ 0x50(SP)(DI*8), R9
  mixing_amd64.go:89            0x14054d92a             4d0fbf09                MOVSX 0(R9), R9
  mixing_amd64.go:89            0x14054d92e             4501c8                  ADDL R9, R8
  mixing_amd64.go:88            0x14054d931             48ffc7                  INCQ DI
  mixing_amd64.go:88            0x14054d934             4839c7                  CMPQ DI, AX
  mixing_amd64.go:88            0x14054d937             7ce9                    JL 0x14054d922
  mixing_amd64.go:86            0x14054d939             4889542430              MOVQ DX, 0x30(SP)
  mixing_amd64.go:91            0x14054d93e             4489c0                  MOVL R8, AX
  mixing_amd64.go:91            0x14054d941             f20f100587952400        MOVSD_XMM $f64.40dfffc000000000(SB), X0
  mixing_amd64.go:91            0x14054d949             e872f9ffff              CALL github.com/gregriff/vogo/cli/internal/audio.softSaturatePade(SB)
  mixing_amd64.go:91            0x14054d94e             488b4c2430              MOVQ 0x30(SP), CX
  mixing_amd64.go:91            0x14054d953             4881f9e0010000          CMPQ CX, $0x1e0
  mixing_amd64.go:91            0x14054d95a             7293                    JB 0x14054d8ef
  mixing_amd64.go:91            0x14054d95c             eb1e                    JMP 0x14054d97c
  mixing_amd64.go:102           0x14054d95e             31c9                    XORL CX, CX
  mixing_amd64.go:102           0x14054d960             eb0c                    JMP 0x14054d96e
  mixing_amd64.go:103           0x14054d962             48c744cc5000000000      MOVQ $0x0, 0x50(SP)(CX*8)
  mixing_amd64.go:102           0x14054d96b             48ffc1                  INCQ CX
  mixing_amd64.go:102           0x14054d96e             4839c1                  CMPQ CX, AX
  mixing_amd64.go:102           0x14054d971             7cef                    JL 0x14054d962
  mixing_amd64.go:105           0x14054d973             4881c4e0000000          ADDQ $0xe0, SP
  mixing_amd64.go:105           0x14054d97a             5d                      POPQ BP
  mixing_amd64.go:105           0x14054d97b             c3                      RET
  mixing_amd64.go:91            0x14054d97c             b8e0010000              MOVL $.file+403(SB), AX
  mixing_amd64.go:91            0x14054d981             e8ba2eb4ff              CALL runtime.panicBounds(SB)
  mixing_amd64.go:82            0x14054d986             e8b52eb4ff              CALL runtime.panicBounds(SB)
  mixing_amd64.go:82            0x14054d98b             b8e0010000              MOVL $.file+403(SB), AX
  mixing_amd64.go:82            0x14054d990             e8ab2eb4ff              CALL runtime.panicBounds(SB)
  mixing_amd64.go:42            0x14054d995             b8e0010000              MOVL $.file+403(SB), AX
  mixing_amd64.go:42            0x14054d99a             e8a12eb4ff              CALL runtime.panicBounds(SB)
  mixing_amd64.go:41            0x14054d99f             90                      NOPL
  mixing_amd64.go:41            0x14054d9a0             e89b2eb4ff              CALL runtime.panicBounds(SB)
  ringbuffer.go:79              0x14054d9a5             e8b6ecafff              CALL runtime.panicdivide(SB)
  ringbuffer.go:76              0x14054d9aa             e8912eb4ff              CALL runtime.panicBounds(SB)
  ringbuffer.go:76              0x14054d9af             b8e0010000              MOVL $.file+403(SB), AX
  ringbuffer.go:76              0x14054d9b4             e8872eb4ff              CALL runtime.panicBounds(SB)
  ringbuffer.go:75              0x14054d9b9             e8822eb4ff              CALL runtime.panicBounds(SB)
  ringbuffer.go:73              0x14054d9be             6690                    NOPW
  ringbuffer.go:73              0x14054d9c0             e87b2eb4ff              CALL runtime.panicBounds(SB)
  ringbuffer.go:73              0x14054d9c5             e8762eb4ff              CALL runtime.panicBounds(SB)
  mixing_amd64.go:16            0x14054d9ca             e8712eb4ff              CALL runtime.panicBounds(SB)
  mixing_amd64.go:16            0x14054d9cf             90                      NOPL
  mixing_amd64.go:9             0x14054d9d0             4889442408              MOVQ AX, 0x8(SP)
  mixing_amd64.go:9             0x14054d9d5             48895c2410              MOVQ BX, 0x10(SP)
  mixing_amd64.go:9             0x14054d9da             e80110b4ff              CALL runtime.morestack_noctxt.abi0(SB)
  mixing_amd64.go:9             0x14054d9df             488b442408              MOVQ 0x8(SP), AX
  mixing_amd64.go:9             0x14054d9e4             488b5c2410              MOVQ 0x10(SP), BX
  mixing_amd64.go:9             0x14054d9e9             e9d2f9ffff              JMP github.com/gregriff/vogo/cli/internal/audio.(*streams).mixAVX512(SB)
  :-1                           0x14054d9ee             cc                      INT $0x3
  :-1                           0x14054d9ef             cc                      INT $0x3
  :-1                           0x14054d9f0             cc                      INT $0x3
  :-1                           0x14054d9f1             cc                      INT $0x3
  :-1                           0x14054d9f2             cc                      INT $0x3
  :-1                           0x14054d9f3             cc                      INT $0x3
  :-1                           0x14054d9f4             cc                      INT $0x3
  :-1                           0x14054d9f5             cc                      INT $0x3
  :-1                           0x14054d9f6             cc                      INT $0x3
  :-1                           0x14054d9f7             cc                      INT $0x3
  :-1                           0x14054d9f8             cc                      INT $0x3
  :-1                           0x14054d9f9             cc                      INT $0x3
  :-1                           0x14054d9fa             cc                      INT $0x3
  :-1                           0x14054d9fb             cc                      INT $0x3
  :-1                           0x14054d9fc             cc                      INT $0x3
  :-1                           0x14054d9fd             cc                      INT $0x3
  :-1                           0x14054d9fe             cc                      INT $0x3
  :-1                           0x14054d9ff             cc                      INT $0x3

TEXT github.com/gregriff/vogo/cli/internal/audio.(*streams).mixAVX512-fm(SB) <autogenerated>
  <autogenerated>:1     0x14054fd20             493b6610                CMPQ SP, 0x10(R14)
  <autogenerated>:1     0x14054fd24             761d                    JBE 0x14054fd43
  <autogenerated>:1     0x14054fd26             55                      PUSHQ BP
  <autogenerated>:1     0x14054fd27             4889e5                  MOVQ SP, BP
  <autogenerated>:1     0x14054fd2a             4883ec10                SUBQ $0x10, SP
  <autogenerated>:1     0x14054fd2e             488b4a08                MOVQ 0x8(DX), CX
  <autogenerated>:1     0x14054fd32             4889c3                  MOVQ AX, BX
  <autogenerated>:1     0x14054fd35             4889c8                  MOVQ CX, AX
  <autogenerated>:1     0x14054fd38             e883d6ffff              CALL github.com/gregriff/vogo/cli/internal/audio.(*streams).mixAVX512(SB)
  <autogenerated>:1     0x14054fd3d             4883c410                ADDQ $0x10, SP
  <autogenerated>:1     0x14054fd41             5d                      POPQ BP
  <autogenerated>:1     0x14054fd42             c3                      RET
  <autogenerated>:1     0x14054fd43             4889442408              MOVQ AX, 0x8(SP)
  <autogenerated>:1     0x14054fd48             e813ecb3ff              CALL runtime.morestack.abi0(SB)
  <autogenerated>:1     0x14054fd4d             488b442408              MOVQ 0x8(SP), AX
  <autogenerated>:1     0x14054fd52             ebcc                    JMP github.com/gregriff/vogo/cli/internal/audio.(*streams).mixAVX512-fm(SB)
  :-1                   0x14054fd54             cc                      INT $0x3
  :-1                   0x14054fd55             cc                      INT $0x3
  :-1                   0x14054fd56             cc                      INT $0x3
  :-1                   0x14054fd57             cc                      INT $0x3
  :-1                   0x14054fd58             cc                      INT $0x3
  :-1                   0x14054fd59             cc                      INT $0x3
  :-1                   0x14054fd5a             cc                      INT $0x3
  :-1                   0x14054fd5b             cc                      INT $0x3
  :-1                   0x14054fd5c             cc                      INT $0x3
  :-1                   0x14054fd5d             cc                      INT $0x3
  :-1                   0x14054fd5e             cc                      INT $0x3
  :-1                   0x14054fd5f             cc                      INT $0x3
