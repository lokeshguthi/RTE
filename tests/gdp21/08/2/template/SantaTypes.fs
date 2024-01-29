module SantaTypes
open Mini

type Point = {
    x: Nat
    y: Nat
}

type Person = {
    name: String
    location: Point
}
