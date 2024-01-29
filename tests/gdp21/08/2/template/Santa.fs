module Santa
open Mini
open SantaTypes

// Beispiele vom Übungsblatt (zur Verwendung im Interpreter)
let lisa  = {name="Lisa" ; location = {x = 1N; y = 2N}}
let alice = {name="Alice"; location = {x = 2N; y = 5N}}
let harry = {name="Harry"; location = {x = 5N; y = 3N}}
let bob   = {name="Bob"  ; location = {x = 4N; y = 4N}}
let santasList = [lisa; alice; harry; bob]

// a)
let distance (a: Person) (b: Person): Nat =
    failwith "TODO"

// b)
let rec pathlength (p: List<Person>): Nat =
    failwith "TODO"

// c)
let rec prepend (elem: 'a) (xss: List<List<'a>>): List<List<'a>> =
    failwith "TODO"

// d)
let rec insert (elem: 'a) (xs: List<'a>): List<List<'a>> =
    failwith "TODO"

// e)
let rec permute (ls: List<'a>): List<List<'a>> =
    failwith "TODO"

// f)
let shortestPath (p: List<Person>): (List<Person> * Nat) =
    failwith "TODO"