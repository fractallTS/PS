# PS
Projektna naloga: Razpravljalnica

Ustvarite distribuirano spletno storitev **Razpravljalnica**, ki bo namenjena izmenjavi mnenj med uporabniki o različnih temah. 

Razpravljalnici se lahko pridružijo novi **uporabniki**, ki nato vanjo dodajajo nove **teme** in znotraj posameznih tem objavljajo **sporočila**. Razpravljalnica omogoča, da se lahko uporabnik na eno ali več tem naroči in sproti prejema iz strežnika sporočila, ki se objavljajo znotraj naročenih tem. Podprto je tudi **všečkanje** sporočil. Za vsako objavljeno sporočilo se beleži število prejetih všečkov. Uporabniki lahko lastna sporočila tudi **urejajo/brišejo**. 

Storitev (strežnik) napišite v programskem jeziku Go. Uporablja naj ogrodje **gRPC** za komunikacijo z odjemalci (uporabniki). Prav tako napišite odjemalca, ki bo znal komunicirati s strežnikom in bo podpiral vse operacije, ki jih ponuja Razpravljalnica. Za komunikacijo znotraj storitve (med strežniki) lahko uporabite poljubno rešitev (rpc). 


Progress:
Osnova je narejena, manjka:
 - storage -> trenutno applyUpdate() samo izpiše na terminal
 - token avtentikacija pri subscribe
 - userji z istim imenom (trenutno ustvari nov userID), mogoče še možnost prijave z geslom
 - graceful exit za subscribe na odjemalcu
 - 2x like naj odstrani like
 - ...


How to Use
1. Start the control server:
cd razpravljalnica && go run . -mode server -role control -clientControlPort :50000 -serverControlPort :50001
2. Start tail server:
cd razpravljalnica && go run . -mode server -role tail -clientPort 50008 -dataPort 50009 -serverControlPort :50001
3. Start head server:
cd razpravljalnica && go run . -mode server -role head -clientPort 50002 -dataPort 50004 -serverControlPort :50001
4. Start chain servers: (for more use different ports)
cd razpravljalnica go run . -mode server -role chain -clientPort 50050 -dataPort 50052

5. Start client:
go run . -mode client -clientMode manual -clientControlPort :50001


