module Tests

open Microsoft.VisualStudio.TestTools.UnitTesting

[<TestClass>]
type Tests() =

    [<TestMethod>] [<Timeout(1000)>]
    member this.``greeting enthält erfolgreich`` (): unit =
        StringAssert.Contains(Program.greeting, "erfolgreich")
