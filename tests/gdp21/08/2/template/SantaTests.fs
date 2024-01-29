module SantaTests

open Microsoft.VisualStudio.TestTools.UnitTesting
open FsCheck
open Mini
open SantaTypes


[<StructuredFormatDisplay("{ToString}")>]
type SafePerson =
    | SP of p: Person
    member this.ToString =
        let (SP p) = this
        sprintf "%A" p

let unwrapSP (SP p) = p

type ArbitraryModifiers =
    static member Nat() =
        Arb.fromGenShrink (
            Gen.choose(0, 100),
            fun n -> seq {
                if n > 0 then yield n - 1
            }
        )
        |> Arb.convert (Nat.Make) (int)

    static member SafePerson() =
        Arb.from<string * Point>
        |> Arb.filter (not << isNull << fst)
        |> Arb.convert (fun (s, p) -> (String.filter (fun c -> c >= 'a' && c <= 'z') s, p)) (id)
        |> Arb.filter (fun (s, _) -> s.Length >= 3)
        |> Arb.convert (fun (s, p) -> SP {name = s; location = p}) (fun (SP person) -> (person.name, person.location))

[<TestClass>]
type Tests() =
    do Arb.register<ArbitraryModifiers>() |> ignore

    // ------------------------------------------------------------------------
    // a)

    [<TestMethod>] [<Timeout(1000)>]
    member this.``a) distance Beispiel 1`` (): unit =
        Assert.AreEqual(7N, Santa.distance {name="Eve";location={x = 1N; y = 2N}} {name="Bob";location={x=2N; y=8N}})

    [<TestMethod>] [<Timeout(1000)>]
    member this.``a) distance Beispiel 2`` (): unit =
        Assert.AreEqual(7N, Santa.distance {name="Eve";location={x = 2N; y = 8N}} {name="Bob";location={x=1N; y=2N}})

    [<TestMethod>] [<Timeout(1000)>]
    member this.``a) distance Beispiel 3`` (): unit =
        Assert.AreEqual(2N, Santa.distance {name="Eve";location={x = 2N; y = 2N}} {name="Bob";location={x=1N; y=1N}})

    [<TestMethod>] [<Timeout(1000)>]
    member this.``a) distance Beispiel 4`` (): unit =
        Assert.AreEqual(18N, Santa.distance {name="Eve";location={x = 30N; y = 7N}} {name="Bob";location={x=15N; y=10N}})

    [<TestMethod>] [<Timeout(10000)>]
    member this.``a) distance Zufall`` (): unit =
        Check.One({Config.QuickThrowOnFailure with EndSize = 10000}, fun (x: SafePerson) (y: SafePerson) ->
            let (x,y) = (unwrapSP x, unwrapSP y)
            Assert.AreEqual(Santa.distance x y, Santa.distance y x)
        )


    // ------------------------------------------------------------------------
    // b)

    [<TestMethod>] [<Timeout(1000)>]
    member this.``b) pathlength Beispiel 1`` (): unit =
        Assert.AreEqual(14N, Santa.pathlength [{name="Eve";location={x = 1N; y = 2N}}; {name="Bob";location={x=2N; y=8N}}])

    [<TestMethod>] [<Timeout(1000)>]
    member this.``b) pathlength Beispiel 2`` (): unit =
        Assert.AreEqual(70N, Santa.pathlength [{name="Eve";location={x = 1N; y = 2N}}; {name="Bob";location={x=2N; y=8N}}; {name="Bill";location={x = 30N; y = 7N}}])

    [<TestMethod>] [<Timeout(10000)>]
    member this.``b) pathlength Zufall 1`` (): unit =
        Check.One({Config.QuickThrowOnFailure with EndSize = 5}, fun (x: SafePerson list) ->
            let x = List.map unwrapSP x
            Assert.AreEqual(Santa.pathlength x, Santa.pathlength (List.rev x), "Hin- und Rückweg sind nicht gleich lang")
        )

    [<TestMethod>] [<Timeout(10000)>]
    member this.``b) pathlength Zufall 2`` (): unit =
        Check.One({Config.QuickThrowOnFailure with EndSize = 5}, fun (x: SafePerson list) ->
            let xs = List.map unwrapSP x
            match xs with
            | [] -> Assert.IsTrue(Santa.pathlength xs >= Santa.pathlength xs)
            | _ :: zs -> Assert.IsTrue(Santa.pathlength xs >= Santa.pathlength zs, "Pfadlänge ohne Anfangspunkt ist nicht kürzer oder gleich der kompletten Pfadlänge")
        )

    [<TestMethod>] [<Timeout(10000)>]
    member this.``b) pathlength Zufall 3`` (): unit =
        Check.One({Config.QuickThrowOnFailure with EndSize = 5}, fun (x: SafePerson list) ->
            let x = List.map unwrapSP x
            let rotate1 (xs : 'a list): 'a list =
                match xs with
                | [] -> []
                | x :: xs' -> xs' @ [x]
            Assert.AreEqual(Santa.pathlength x, Santa.pathlength (rotate1 x), "Pfadlänge ändert sich unter zyklischer Vertauschung der Wegpunkte! Kehrt Santa am Ende des Pfades zum Ausgangspunkt zurück?")
        )

    // ------------------------------------------------------------------------
    // c)
    [<TestMethod>] [<Timeout(1000)>]
    member this.``c) prepend Beispiel`` (): unit =
        Assert.AreEqual([[1N; 2N; 3N]; [1N; 4N; 5N]], List.sort (Santa.prepend 1N [[2N; 3N]; [4N; 5N]]))

    [<TestMethod>] [<Timeout(1000)>]
    member this.``c) prepend Zufall`` (): unit =
        Check.One({Config.QuickThrowOnFailure with EndSize = 1000}, fun (elem: Nat) (xss: Nat list list) ->
            Assert.AreEqual(elem * Nat.Make (List.length xss) + List.sumBy (List.sum) xss,  List.sumBy (List.sum) (Santa.prepend elem xss), "elem wird nicht genau ein mal in jede Liste von xss eingefügt")
        )


    // ------------------------------------------------------------------------
    // d)

    [<TestMethod>] [<Timeout(1000)>]
    member this.``d) insert Beispiel`` (): unit =
        Assert.AreEqual(List.sort([[1N;2N;3N];[2N;1N;3N];[2N;3N;1N]]), List.sort (Santa.insert 1N [2N;3N]))


    [<TestMethod>] [<Timeout(10000)>]
    member this.``d) insert Zufall`` (): unit =
        Check.One({Config.QuickThrowOnFailure with EndSize = 1000}, fun (x: Nat) (xs: Nat list) ->
            let inslist = Santa.insert x xs
            Assert.AreEqual(List.length xs + 1, List.length inslist)
            Assert.IsTrue(List.contains (x :: xs) inslist, "insert x xs enthält x::xs nicht")
            Assert.IsTrue(List.contains (xs @ [x]) inslist, "insert x xs enthält xs @ [x] nicht")
        )

    // ------------------------------------------------------------------------
    // e)

    [<TestMethod>] [<Timeout(10000)>]
    member this.``e) permute Beispiel`` (): unit =
        Assert.AreEqual(List.sort([[1N;2N;3N];[1N;3N;2N];[2N;1N;3N];[2N;3N;1N];[3N;1N;2N];[3N;2N;1N]]), List.sort (Santa.permute [1N;2N;3N]))


    [<TestMethod>] [<Timeout(10000)>]
    member this.``e) permute Zufall 1`` (): unit =
        Check.One({Config.QuickThrowOnFailure with MaxTest = 500; EndSize = 5}, fun (xs: Nat list) ->
            let fact n = [1..n] |> List.fold (*) 1
            let perms = (Santa.permute xs)
            Assert.AreEqual(fact (List.length xs), List.length perms, "Die Liste der permutierten Listen enthält nicht alle oder zu viele Elemente")
        )

    [<TestMethod>] [<Timeout(10000)>]
    member this.``e) permute Zufall 2`` (): unit =
        Check.One({Config.QuickThrowOnFailure with MaxTest = 500; EndSize = 5}, fun (xs: Nat list) ->
            let fact n = [1..n] |> List.fold (*) 1
            let perms = (Santa.permute xs)
            Assert.IsTrue(List.contains xs perms, "Die permutierte Liste enthalt die Eingabeliste nicht")
        )

    [<TestMethod>] [<Timeout(10000)>]
    member this.``e) permute Zufall 3`` (): unit =
        Check.One({Config.QuickThrowOnFailure with MaxTest = 500; EndSize = 5}, fun (xs: Nat list) ->
            let fact n = [1..n] |> List.fold (*) 1
            let perms = (Santa.permute xs)
            Assert.IsTrue(List.contains (List.rev xs) perms, "Die permutierte Liste enthält die umgekehrte Eingabeliste nicht")
        )

    // ------------------------------------------------------------------------
    // f)

    [<TestMethod>] [<Timeout(10000)>]
    member this.``f) shortestPath Beispiel`` (): unit =
        Assert.AreEqual(4N, snd (Santa.shortestPath [{name="Eve";location={x=0N;y=0N}}; {name="Bob";location={x=1N;y=0N}}; {name="Bill";location={x=1N;y=1N}}; {name="Alice";location={x=0N;y=1N}}]))

    [<TestMethod>] [<Timeout(10000)>]
    member this.``f) shortestPath Beispiel2`` (): unit =
        Assert.AreEqual(4N, snd (Santa.shortestPath [{name="Eve";location={x=0N;y=0N}}; {name="Bill";location={x=1N;y=1N}}; {name="Bob";location={x=1N;y=0N}}; {name="Alice";location={x=0N;y=1N}}]))

    [<TestMethod>] [<Timeout(10000)>]
    member this.``f) shortestPath Zufall`` (): unit =
        Check.One({Config.QuickThrowOnFailure with MaxTest = 500; EndSize = 5}, fun (xs: SafePerson list) ->
            let xs = List.map unwrapSP xs
            Assert.IsTrue( snd (Santa.shortestPath xs) <= Santa.pathlength xs , "Die Länge des kürzesten Pfades ist nicht kleiner oder gleich der Länge des Eingabepfades")
        )
