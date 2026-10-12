module View.Dashboard exposing
    ( ActivityEntry
    , PriorityAlert
    , activityPanel
    , alertsPanel
    , caseEntry
    , closingSoonAlert
    , dealEntry
    , documentCorrectionAlert
    , documentEntry
    , expiredDocumentAlert
    , invoiceEntry
    , leadEntry
    , outstandingInvoiceAlert
    , overdueTaskAlert
    , paymentEntry
    , pendingAgentAlert
    , pendingSchoolAlert
    , staleDealAlert
    , studentEntry
    , taskEntry
    , urgentCaseAlert
    )

{-| Shared Priority Alerts + Board Activity panel components.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class)
import Html.Events exposing (onClick)
import Types exposing (..)
import View.Format exposing (dateToDays)
import View.Helpers exposing (onEnter)


type alias PriorityAlert =
    { severity : String
    , icon : String
    , headline : String
    , body : String
    , meta : String
    , action : Msg
    }


type alias ActivityEntry =
    { createdAt : String
    , view : Html Msg
    }


isPast : String -> String -> Bool
isPast today dateStr =
    case ( dateToDays today, dateToDays dateStr ) of
        ( Just t, Just d ) ->
            d < t

        _ ->
            False


daysBetween : String -> String -> Maybe Int
daysBetween a b =
    Maybe.map2 (\x y -> y - x) (dateToDays a) (dateToDays b)


alertItemView : PriorityAlert -> Html Msg
alertItemView alert =
    div
        [ class ("alert-item " ++ alert.severity)
        , Attr.attribute "role" "button"
        , Attr.attribute "tabindex" "0"
        , onEnter alert.action
        , onClick alert.action
        ]
        [ div [ class "alert-icon" ] [ text alert.icon ]
        , div []
            [ div [ class "alert-text" ]
                [ strong [] [ text alert.headline ]
                , text (" — " ++ alert.body)
                ]
            , div [ class "alert-meta" ] [ text alert.meta ]
            ]
        ]


activityRow : String -> Msg -> String -> String -> String -> Html Msg
activityRow badgeText msg kindText timeText titleText =
    div
        [ class "activity-item"
        , Attr.attribute "role" "button"
        , Attr.attribute "tabindex" "0"
        , onEnter msg
        , onClick msg
        ]
        [ div [ class "activity-item__marker" ] []
        , div [ class "activity-item__body" ]
            [ div [ class "activity-item__title-row" ]
                [ h4 [ class "activity-item__title" ] [ text titleText ]
                , span [ class "badge badge--info" ] [ text badgeText ]
                ]
            , div [ class "activity-item__meta" ]
                [ span [ class "activity-item__kind" ] [ text kindText ]
                , span [ class "activity-item__time" ] [ text timeText ]
                ]
            ]
        ]


alertsPanel : List PriorityAlert -> Html Msg
alertsPanel alerts =
    div [ class "side-card" ]
        [ h3 [] [ text "Priority alerts" ]
        , if List.isEmpty alerts then
            p [ class "detail-muted" ]
                [ text "Nothing urgent. The board is clear." ]

          else
            div [] (List.map alertItemView (List.take 4 alerts))
        ]


activityPanel : List ActivityEntry -> Html Msg
activityPanel entries =
    let
        sorted =
            entries
                |> List.filter (\e -> not (String.isEmpty e.createdAt))
                |> List.sortBy .createdAt
                |> List.reverse
                |> List.take 6
                |> List.map .view
    in
    div [ class "side-card" ]
        [ h3 [] [ text "Recent activity" ]
        , if List.isEmpty sorted then
            p [ class "detail-muted" ]
                [ text "No recent records yet." ]

          else
            div [ class "activity-list" ] sorted
        ]


urgentCaseAlert : Case -> PriorityAlert
urgentCaseAlert c =
    { severity = "urgent"
    , icon = "!"
    , headline = c.caseNumber
    , body = c.serviceCategory ++ " marked urgent"
    , meta = "Application · " ++ c.destinationCountry
    , action = OpenedCaseDetail c
    }


overdueTaskAlert : Task -> PriorityAlert
overdueTaskAlert t =
    { severity = "urgent"
    , icon = "T"
    , headline = t.title
    , body = "overdue"
    , meta = "Task · due " ++ t.dueDate
    , action = NavigatedTo Tasks
    }


outstandingInvoiceAlert : Invoice -> PriorityAlert
outstandingInvoiceAlert inv =
    { severity = ""
    , icon = "$"
    , headline = inv.invoiceNumber
    , body = "outstanding balance"
    , meta = "Finance · " ++ inv.clientName
    , action = OpenedInvoiceDetail inv
    }


pendingSchoolAlert : School -> PriorityAlert
pendingSchoolAlert s =
    { severity = ""
    , icon = "S"
    , headline = s.name
    , body = "contract pending"
    , meta = "School · " ++ s.countryCode
    , action = OpenedSchoolDetail s
    }


pendingAgentAlert : Agent -> PriorityAlert
pendingAgentAlert a =
    { severity = ""
    , icon = "A"
    , headline = a.name
    , body = "contract not signed"
    , meta = "Agent · " ++ a.countryCode
    , action = OpenedAgentDetail a
    }


documentCorrectionAlert : Document -> PriorityAlert
documentCorrectionAlert d =
    { severity = "urgent"
    , icon = "D"
    , headline = d.docName
    , body = "correction required"
    , meta = "Document · " ++ d.caseNumber
    , action = NavigatedTo Cases
    }


expiredDocumentAlert : Document -> PriorityAlert
expiredDocumentAlert d =
    { severity = ""
    , icon = "D"
    , headline = d.docName
    , body = "expired"
    , meta = "Document · " ++ d.caseNumber
    , action = NavigatedTo Cases
    }


staleDealAlert : String -> Deal -> PriorityAlert
staleDealAlert today d =
    let
        days =
            daysBetween d.createdAt today
                |> Maybe.withDefault 0
    in
    { severity = "urgent"
    , icon = "!"
    , headline = d.title
    , body = "no movement in " ++ String.fromInt days ++ " days"
    , meta = "Deal · " ++ d.stage
    , action = OpenedDealDetail d
    }


closingSoonAlert : Deal -> PriorityAlert
closingSoonAlert d =
    { severity = "urgent"
    , icon = "!"
    , headline = d.title
    , body = "closes " ++ d.closeDate
    , meta = "Deal · " ++ d.stage
    , action = OpenedDealDetail d
    }


leadEntry : Lead -> ActivityEntry
leadEntry l =
    { createdAt = l.createdAt
    , view =
        activityRow "Lead"
            (OpenedLeadDetail l)
            (if String.isEmpty l.interestedCountry then
                "—"

             else
                l.interestedCountry
            )
            (String.left 10 l.createdAt)
            l.name
    }


caseEntry : Case -> ActivityEntry
caseEntry c =
    { createdAt = c.createdAt
    , view =
        activityRow "Application"
            (OpenedCaseDetail c)
            (c.destinationCountry ++ " · " ++ c.currentStage)
            (String.left 10 c.createdAt)
            c.caseNumber
    }


studentEntry : Student -> ActivityEntry
studentEntry s =
    { createdAt = s.createdAt
    , view =
        activityRow "Student"
            (OpenedStudentDetail s)
            (if String.isEmpty s.program then
                s.schoolName

             else
                s.program
            )
            (String.left 10 s.createdAt)
            s.name
    }


dealEntry : Deal -> ActivityEntry
dealEntry d =
    { createdAt = d.createdAt
    , view =
        activityRow "Deal"
            (OpenedDealDetail d)
            (d.stage
                ++ (if String.isEmpty d.owner then
                        ""

                    else
                        " · " ++ d.owner
                   )
            )
            (String.left 10 d.createdAt)
            d.title
    }


taskEntry : Task -> ActivityEntry
taskEntry t =
    { createdAt = t.createdAt
    , view =
        activityRow "Task"
            (NavigatedTo Tasks)
            (t.status
                ++ (if String.isEmpty t.dueDate then
                        ""

                    else
                        " · due " ++ t.dueDate
                   )
            )
            (String.left 10 t.createdAt)
            t.title
    }


documentEntry : Document -> ActivityEntry
documentEntry d =
    { createdAt = d.createdAt
    , view =
        activityRow "Document"
            (NavigatedTo Cases)
            (d.status
                ++ (if String.isEmpty d.caseNumber then
                        ""

                    else
                        " · " ++ d.caseNumber
                   )
            )
            (String.left 10 d.createdAt)
            d.docName
    }


invoiceEntry : Invoice -> ActivityEntry
invoiceEntry inv =
    { createdAt = inv.createdAt
    , view =
        activityRow "Invoice"
            (OpenedInvoiceDetail inv)
            (inv.paymentMilestone
                ++ (if String.isEmpty inv.clientName then
                        ""

                    else
                        " · " ++ inv.clientName
                   )
            )
            (String.left 10 inv.createdAt)
            inv.invoiceNumber
    }


paymentEntry : Payment -> ActivityEntry
paymentEntry p =
    { createdAt = p.createdAt
    , view =
        activityRow "Payment"
            (NavigatedTo Invoices)
            (p.method
                ++ (if String.isEmpty p.paidOn then
                        ""

                    else
                        " · " ++ p.paidOn
                   )
            )
            (String.left 10 p.createdAt)
            (String.fromFloat p.amount ++ " " ++ p.reference)
    }
