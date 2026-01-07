# PS
Projektna naloga: Razpravljalnica

Ustvarite distribuirano spletno storitev **Razpravljalnica**, ki bo namenjena izmenjavi mnenj med uporabniki o različnih temah. 

Razpravljalnici se lahko pridružijo novi **uporabniki**, ki nato vanjo dodajajo nove **teme** in znotraj posameznih tem objavljajo **sporočila**. Razpravljalnica omogoča, da se lahko uporabnik na eno ali več tem naroči in sproti prejema iz strežnika sporočila, ki se objavljajo znotraj naročenih tem. Podprto je tudi **všečkanje** sporočil. Za vsako objavljeno sporočilo se beleži število prejetih všečkov. Uporabniki lahko lastna sporočila tudi **urejajo/brišejo**. 

Storitev (strežnik) napišite v programskem jeziku Go. Uporablja naj ogrodje **gRPC** za komunikacijo z odjemalci (uporabniki). Prav tako napišite odjemalca, ki bo znal komunicirati s strežnikom in bo podpiral vse operacije, ki jih ponuja Razpravljalnica. Za komunikacijo znotraj storitve (med strežniki) lahko uporabite poljubno rešitev (rpc). 


Progress:
Osnova je narejena, manjka:
 - interface za odjemalca(CLI) -> naceloma narejen (zazene se isto kot odjemalec sam se flag -clientMode manual)
 - pravilna control plane implementacija
 - verižna replikacija (interNode komunikacija, )
 - token avtentikacija pri subscribe
 - userji z istim imenom (trenutno ustvari nov userID), mogoče še možnost prijave z geslom
 - graceful exit za subscribe na odjemalcu
 - 2x like naj odstrani like
 - node vpraša control plane za naslednji node pri replikaciji
 - ...

How to Use
Start the server:
cd razpravljalnica && go run . -mode server -address :50051
Run the client:
go run . -mode client -clientMode test -address localhost:50051