@echo off
cd razpravljalnica
start "Control Server" cmd /k "go run . -mode server -role control -clientControlPort :50000 -serverControlPort :50001"
timeout /t 3 /nobreak > nul
start "Tail Server" cmd /k "go run . -mode server -role tail -clientPort 50008 -dataPort 50009 -serverControlPort :50001"
timeout /t 2 /nobreak > nul
start "Head Server" cmd /k "go run . -mode server -role head -clientPort 50002 -dataPort 50004 -serverControlPort :50001"
timeout /t 2 /nobreak > nul
start "Chain Server 1" cmd /k "go run . -mode server -role chain -clientPort 50050 -dataPort 50052"
timeout /t 2 /nobreak > nul
start "Chain Server 2" cmd /k "go run . -mode server -role chain -clientPort 50054 -dataPort 50056"
timeout /t 2 /nobreak > nul
start "Client" cmd /k "go run . -mode client -clientMode manual -clientControlPort :50001"