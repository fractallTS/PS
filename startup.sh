#!/bin/bash
cd razpravljalnica

# Funkcija za odpiranje novega terminala z določenim naslovom in ukazom
open_terminal() {
    gnome-terminal --title="$1" -- bash -c "$2; bash"
}

open_terminal "Control Server" "go run . -mode server -role control -clientControlPort :50000 -serverControlPort :50001"
sleep 4
open_terminal "Tail Server" "go run . -mode server -role tail -clientPort 50008 -dataPort 50009 -serverControlPort :50001"
open_terminal "Head Server" "go run . -mode server -role head -clientPort 50002 -dataPort 50004 -serverControlPort :50001"
open_terminal "Chain Server 1" "go run . -mode server -role chain -clientPort 50050 -dataPort 50052"
open_terminal "Chain Server 2" "go run . -mode server -role chain -clientPort 50054 -dataPort 50056"
open_terminal "Client 1" "go run . -mode client -clientMode manual -clientControlPort :50000"
open_terminal "Client 2" "go run . -mode client -clientMode manual -clientControlPort :50000"
