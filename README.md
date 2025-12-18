# PS
Projektna naloga: Razpravljalnica

Ustvarite distribuirano spletno storitev **Razpravljalnica**, ki bo namenjena izmenjavi mnenj med uporabniki o različnih temah. 

Razpravljalnici se lahko pridružijo novi **uporabniki**, ki nato vanjo dodajajo nove **teme** in znotraj posameznih tem objavljajo **sporočila**. Razpravljalnica omogoča, da se lahko uporabnik na eno ali več tem naroči in sproti prejema iz strežnika sporočila, ki se objavljajo znotraj naročenih tem. Podprto je tudi **všečkanje** sporočil. Za vsako objavljeno sporočilo se beleži število prejetih všečkov. Uporabniki lahko lastna sporočila tudi **urejajo/brišejo**. 

Storitev (strežnik) napišite v programskem jeziku Go. Uporablja naj ogrodje **gRPC** za komunikacijo z odjemalci (uporabniki). Prav tako napišite odjemalca, ki bo znal komunicirati s strežnikom in bo podpiral vse operacije, ki jih ponuja Razpravljalnica. Za komunikacijo znotraj storitve (med strežniki) lahko uporabite poljubno rešitev (rpc). 
