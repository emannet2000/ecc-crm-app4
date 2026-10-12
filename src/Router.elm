module Router exposing (parseRoute, routeToPath)

{-| URL parsing and route <-> path conversion.
-}

import Types exposing (..)
import Url
import Url.Parser as Parser exposing ((</>), oneOf, s, string)


routeParser : Parser.Parser (Route -> a) a
routeParser =
    oneOf
        [ Parser.map Home Parser.top
        , Parser.map Contacts (s "contacts")
        , Parser.map ContactDetail (s "contacts" </> string)
        , Parser.map Deals (s "deals")
        , Parser.map DealDetail (s "deals" </> string)
        , Parser.map Tasks (s "tasks")
        , Parser.map Reports (s "reports")
        , Parser.map Chat (s "chat")
        , Parser.map Workspace (s "workspace")
        , Parser.map Administration (s "administration")
        , Parser.map Settings (s "settings")
        , Parser.map Schools (s "schools")
        , Parser.map SchoolDetail (s "schools" </> string)
        , Parser.map Students (s "students")
        , Parser.map StudentDetail (s "students" </> string)
        , Parser.map Agents (s "agents")
        , Parser.map AgentDetail (s "agents" </> string)
        , Parser.map Leads (s "leads")
        , Parser.map LeadDetail (s "leads" </> string)
        , Parser.map Cases (s "cases")
        , Parser.map CaseDetail (s "cases" </> string)
        , Parser.map Invoices (s "invoices")
        , Parser.map InvoiceDetail (s "invoices" </> string)
        , Parser.map Partners (s "partners")
        , Parser.map PartnerDetail (s "partners" </> string)
        ]


parseRoute : Url.Url -> Route
parseRoute url =
    Maybe.withDefault Home (Parser.parse routeParser url)


routeToPath : Route -> String
routeToPath route =
    case route of
        Home ->
            "/"

        Contacts ->
            "/contacts"

        ContactDetail id ->
            "/contacts/" ++ id

        Deals ->
            "/deals"

        DealDetail id ->
            "/deals/" ++ id

        Tasks ->
            "/tasks"

        Chat ->
            "/chat"

        Workspace ->
            "/workspace"

        Administration ->
            "/administration"

        Reports ->
            "/reports"

        Settings ->
            "/settings"

        Schools ->
            "/schools"

        SchoolDetail id ->
            "/schools/" ++ id

        Students ->
            "/students"

        StudentDetail id ->
            "/students/" ++ id

        Agents ->
            "/agents"

        AgentDetail id ->
            "/agents/" ++ id

        Leads ->
            "/leads"

        LeadDetail id ->
            "/leads/" ++ id

        Cases ->
            "/cases"

        CaseDetail id ->
            "/cases/" ++ id

        Invoices ->
            "/invoices"

        InvoiceDetail id ->
            "/invoices/" ++ id

        Partners ->
            "/partners"

        PartnerDetail id ->
            "/partners/" ++ id
