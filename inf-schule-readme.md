# Installation

## Benötigte Software auf Server

- Docker
    - Zum ausführen der Tests wird das Python Image benötigt: 
    
            docker pull peterzel/pydev
        
- Python3 (für die Vergleichtools)

## RTE Service

RTE (Remote Test Executor) muss als Service installiert werden.

Auf unserem Server ist das zum Beispiel über einen SystemD Service installiert:

- Unter `/opt/school-rte/rte` liegt die Executable.
- Unter `/opt/school-rte/runs` werden die einzelnen Test-runs abgelegt.
- Unter `/opt/school-rte/tests` befinden sich die Testfälle.
- Unter `/opt/school-rte/tests/_tools` befinden sich die Tools zum Vergleichen von Ausgaben bei IO-Tests.
- Ein Benutzer `rte` wurde angelegt, der Rechte hat Docker auszuführen (siehe [Manage Docker as a non-root user](https://docs.docker.com/install/linux/linux-postinstall/#manage-docker-as-a-non-root-user)).
- Unter `/lib/systemd/system/school-rte.service` wurde ein Service für RTE wie folgt definiert:

        [Unit]
        Description=Remote Test Executor for Schools Demo
        After=network.target
        
        [Service]
        WorkingDirectory=/opt/school-rte
        ExecStart=/opt/school-rte/rte --port 8083 --docker_image_python peterzel/pydev --metricson :3004
        User=rte
        
        [Install]
        WantedBy=multi-user.target

- Die verwendeten Ordner können über Programmparameter noch eingestellt werden. 
    Hier eine Übersicht über die Optionen (Ausgabe von `rte --help`):

      -basedir string
            the base directory (tests, runs, ...) (default ".")
      -clean_testruns
            Remove test run folders after executing tests.
      -contextPath string
            A prefix that is used for all URLs on the server.
      -debug
            Turn on debug logging
      -docker_image_c string
            Image to use for C tests. (default "softech/cdev")
      -docker_image_fsharp string
            Image to use for F# tests. (default "softech/fsharpdev")
      -docker_image_java string
            Image to use for Java tests. (default "openjdk:8")
      -docker_image_python string
            Image to use for Python tests. (default "softech/pydev")
      -host string
            hostname the program should bind to
      -metricson string
            port the program should expose Prometheus metrics on (default ":3003")
      -port int
            port the program should listen on (default 8080)
      -testName string
            Testname of a specific solution to test. Use with 'testSolution'.
      -testSolution
            Test the solutions stored in the test directory.
      -testdata_folder string
            Folder where tests are stored. If this is not an absolute path it is interpreted relative to the basedir. (default "tests")
      -testrun_folder string
            Folder where individual test runs are stored. If this is not an absolute path it is interpreted relative to the basedir. (default "runs")
      -tools_folder string
            Folder where individual test runs are stored. If this is not an absolute path it is interpreted relative to the testdata_folder. (default "_tools")
    
- Nach der Installation:

    - Starten mit `sudo systemctl start school-rte`
    - Stoppen mit `sudo systemctl stop school-rte`
    - Logs anzeigen mit `sudo journalctl -u school-rte`
    
## Web-server einrichten

Um eine Verbindung über HTTPS zu bekommen, kann in Apache eine Weiterleitung eingerichtet werden:

Bei uns waren das die folgenden beiden Zeilen in der Apache Konfiguration (`/etc/apache2/sites-available/ssl.conf`):


    ProxyPass        /inf-schule-rte/ http://lamport.cs.uni-kl.de:8083/
    ProxyPassReverse /inf-schule-rte/ http://lamport.cs.uni-kl.de:8083/

Zum Testen: Nach der Einrichtung sollte man unter `https://softech.cs.uni-kl.de/inf-schule-rte/listtests` eine Auflistung der verfügbaren Tests erhalten.

## Einbindung in Seite

- Die URL in der ersten Zeile von `rte.js` muss entsprechend angepasst werden.
- Die JavaScript Dateien `dropzone.js` und `rte.js` und die entsprechenden `css` Stylesheets müssen in die Seite eingebunden werden.

        <script src="./js/dropzone.js"></script>
        <script src="./js/rte.js"></script>
        <link rel="stylesheet" href="./css/dropzone.css">
        <link rel="stylesheet" href="./css/rte.css">

- Ein Test kann dann mit dem folgenden HTML Snippet eingebunden werden:

        <div class="rtetest" data-testname="inf-schule/Demo/1">
            Das Test-Skript wurde noch nicht geladen (Es wird JavaScript benötigt).
        </div>
        
    Der Pfad unter `data-testname` ist der Pfad unter dem sich die Tests auf dem Server befinden (wenn die Tests unter `/opt/school-rte/tests` liegen, dann muss im Beispiel oben also eine Datei `/opt/school-rte/tests/inf-schule/Demo/1/config.json` liegen).
    
- Alternativ zum Upload-Feld kann der Test auch mit einem Editor direkt auf der Seite eingebunden werden.
    Dazu kann die folgende Vorlage verwendet werden:
    
        <div class="rtetest" data-testname="inf-schule/Demo/3" data-filename="bruchoperationen.py">
            <pre>
                Hier kann eine Vorlage für den Code stehen.
            </pre>
        </div>
        
        Das Attribut `data-filename` entspricht dann dem Dateinamen unter dem der Inhalt des Editors hochgeladen werden soll.
        
    Zur Verwendung des Editors muss dieser außerdem noch in die Seite eingebunden werden:
    
        <script src="./js/ace/ace.js"></script>    
    
# Die Tests

Für jeden Test muss es eine `config.json` Datei geben, die den Test beschreibt.

Beispiele und Vergleich Tools gibt es unter https://softech-git.informatik.uni-kl.de/ag/exclaim-inf-schule


Optional können da Grenzen für die Ausführung konfiguriert werden:

 - Timeout: Maximale Ausführungszeit in Sekunden (Default ist 10 Sekunden)
 - MaxMem: Maximaler Speicherverbrauch in MB (Default ist 100MB)

Es gibt zwei Arten von Tests mit unterschiedlichen Konfigurationsmöglichkeiten:

## Input/Output Tests

Beispiel-Konfiguration für einen IO-Test:

    {
        "Compiler": "PythonCompiler",
        "CompareTool": "wildcard_compare.py"
    }

Für jeden Testfall gibt es neben der `config.json` Datei eine Datei  `testname.in.txt` und eine Datei `testname.out.txt`. 
Das hochgeladene Programm wird ausgeführt und erhält den Inhalt von `testname.in.txt` über die Standard-Eingabe.
Anschließend wird die Ausgabe des Programms mit der erwarteten Ausgabe in `testname.out.txt` verglichen.

Optional kann noch eine Datei mit Namen `testname.param.txt` angelegt werden, in der Programmparameter für das Programm definiert werden. 


Mit der Option `CompareTool` kann ein Tool aus dem `_tools` Ordner verwendet werden um die tatsächliche mit der erwarteten Ausgabe zu vergleichen. 
Das Tool kann ein beliebiges Programm sein, dass zwei Dateinamen (zuerst erwartete Ausgabe, dann tatsächliche Ausgabe) als Programmparameter erhält.
Wenn der Vergleich fehlschlägt (Exitcode ungleich `0`), dann wird die Ausgabe des Vergleich-Programms als erwartete Ausgabe angezeigt.

Das Tool `wildcard_compare.py` erlaubt es in der erwarteten Ausgabe drei Punkte (`...`) zu verwenden, für die dann beliebiger Text in der tatsächlichen Ausgabe vorkommen kann.


## Unit Tests

Beispiel-Konfiguration für einen Unit-Test:

    {
        "Compiler": "PythonCompiler",
        "TestType": "PyTest"
    }

Für Unittests wird das [pytest Modul](https://pytest.org) verwendet.
Es wird mit den folgenden Optionen aufgerufen:

    pytest -v --junitxml=./test-result.xml --doctest-glob='*.md' --doctest-modules
    
Das bedeutet, dass alle Doctests in Modulen, alle Doctests in Markdown-Dateien (`*.md`) und alle Unit tests (siehe [unittest Modul](https://docs.python.org/3/library/unittest.html)) ausgeführt werden.


