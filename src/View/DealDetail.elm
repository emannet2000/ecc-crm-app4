module View.DealDetail exposing (dealDetailView)

{-| Deal detail page.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class, disabled, type_)
import Html.Events exposing (onClick)
import Types exposing (..)
import View.StaffPicker as StaffPicker
import View.Contacts exposing (contactById, contactList, stageBadge)
import View.Dashboard exposing (ActivityEntry, PriorityAlert, activityPanel, alertsPanel, closingSoonAlert, dealEntry, overdueTaskAlert, staleDealAlert)
import View.Deals exposing (dealStageOptions)
import View.Format exposing (dateToDays, dealAgeDays, dealAgeLabel, formatCurrencyWith)
import View.Helpers exposing (detailCard, detailEmpty, detailStat, infoRow, initials)
import View.Icons exposing (iconBack, iconCalendar, iconDeals, iconEdit, iconTasks, iconTrash, iconUserTiny)
import View.Workflow exposing (workflowView)


dealStagePillsDetail : Bool -> Deal -> Html Msg
dealStagePillsDetail isMoving d =
    div [ class "stage-pills stage-pills--detail" ]
        (List.map
            (\s ->
                let
                    cls =
                        if d.stage == s then
                            "stage-pill stage-pill--active"

                        else
                            "stage-pill"
                in
                button
                    [ type_ "button"
                    , class cls
                    , onClick (MovedDeal d s)
                    , disabled isMoving
                    ]
                    [ text s ]
            )
            dealStageOptions
        )


dealAlerts : Model -> Deal -> List PriorityAlert
dealAlerts model d =
    let
        isOpen =
            d.stage /= "Won" && d.stage /= "Lost"

        staleAlert =
            if isOpen then
                let
                    age =
                        dealAgeDays model.today d.createdAt
                            |> Maybe.withDefault 0
                in
                if age >= 45 then
                    [ staleDealAlert model.today d ]

                else
                    []

            else
                []

        closingAlert =
            if isOpen && not (String.isEmpty d.closeDate) then
                case ( dateToDays model.today, dateToDays d.closeDate ) of
                    ( Just t, Just c ) ->
                        if c - t <= 30 && c >= t then
                            [ closingSoonAlert d ]

                        else
                            []

                    _ ->
                        []

            else
                []

        fromTasks =
            case ( model.tasks, d.contactId ) of
                ( Success data, cid ) ->
                    if String.isEmpty cid then
                        []

                    else
                        data.items
                            |> List.filter (\t -> t.contactId == cid && t.status /= "done")
                            |> List.map overdueTaskAlert

                _ ->
                    []
    in
    fromTasks ++ staleAlert ++ closingAlert


dealActivity : Model -> Deal -> List ActivityEntry
dealActivity model d =
    case model.deals of
        Success data ->
            if String.isEmpty d.contactId then
                []

            else
                data.items
                    |> List.filter (\x -> x.contactId == d.contactId && x.id /= d.id)
                    |> List.map dealEntry

        _ ->
            []


dealDetailView : Model -> Deal -> Html Msg
dealDetailView model d =
    let
        contacts =
            contactList model

        linkedContact =
            contactById contacts d.contactId

        isMoving =
            model.movingDealId == Just d.id

        closeDateDisplay =
            if String.isEmpty d.closeDate then
                "—"

            else
                d.closeDate

        ownerDisplay =
            if String.isEmpty d.owner then
                "Unassigned"

            else
                d.owner

        createdDisplay =
            if String.isEmpty d.createdAt then
                "—"

            else
                d.createdAt

        ageDays =
            dealAgeDays model.today d.createdAt

        ageLabel =
            dealAgeLabel ageDays
    in
    div [ class "detail" ]
        [ button
            [ class "detail__back"
            , type_ "button"
            , onClick (NavigatedTo Deals)
            ]
            [ iconBack
            , span [] [ text "Back to deals" ]
            ]
        , header [ class "detail-hero" ]
            [ div [ class "detail-hero__avatar detail-hero__avatar--deal" ]
                [ text (formatCurrencyWith d.currency d.value) ]
            , div [ class "detail-hero__body" ]
                [ div [ class "detail-hero__title-row" ]
                    [ h1 [ class "detail-hero__name" ] [ text d.title ]
                    , stageBadge d.stage
                    ]
                , p [ class "detail-hero__role" ]
                    [ text
                        (if String.isEmpty d.contactName then
                            "No contact linked"

                         else
                            "Linked to " ++ d.contactName
                        )
                    ]
                , div [ class "detail-hero__contact" ]
                    [ if String.isEmpty d.closeDate then
                        text ""

                      else
                        span [ class "detail-hero__chip" ]
                            [ iconCalendar
                            , span [] [ text ("Closes " ++ d.closeDate) ]
                            ]
                    , if String.isEmpty d.owner then
                        text ""

                      else
                        span [ class "detail-hero__chip" ]
                            [ iconUserTiny
                            , span [] [ StaffPicker.name d.owner ]
                            ]
                    ]
                ]
            , div [ class "detail-hero__actions" ]
                [ button
                    [ class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , type_ "button"
                    , onClick (OpenedEditDeal d)
                    ]
                    [ iconEdit
                    , span [] [ text "Edit" ]
                    ]
                , button
                    [ class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , type_ "button"
                    , onClick (RequestedDeleteDeal d)
                    ]
                    [ iconTrash
                    , span [] [ text "Delete" ]
                    ]
                ]
            ]
        , div [ class "detail-stats" ]
            [ detailStat "Value" (formatCurrencyWith d.currency d.value) "Deal value"
            , detailStat "Stage" d.stage "Current stage"
            , detailStat "Close date" closeDateDisplay "Expected"
            , detailStat "Age" ageLabel "Since created"
            ]
        , div [ class "detail__grid" ]
            [ aside [ class "detail__sidebar" ]
                [ detailCard "Contact"
                    Nothing
                    (case linkedContact of
                        Just c ->
                            div
                                [ class "deal-contact-card"
                                , Attr.attribute "role" "button"
                                , Attr.attribute "tabindex" "0"
                                , Attr.title ("Open " ++ c.name)
                                , onClick (OpenedContactDetail c)
                                ]
                                [ div [ class "deal-contact-card__avatar" ]
                                    [ text (initials c.name) ]
                                , div [ class "deal-contact-card__body" ]
                                    [ span [ class "deal-contact-card__name" ]
                                        [ text c.name ]
                                    , span [ class "deal-contact-card__email" ]
                                        [ text c.email ]
                                    ]
                                , span [ class "deal-contact-card__chevron" ]
                                    [ text "→" ]
                                ]

                        Nothing ->
                            if String.isEmpty d.contactName then
                                p [ class "detail-muted" ]
                                    [ text "No contact linked to this deal." ]

                            else
                                p [ class "detail-muted" ]
                                    [ text d.contactName ]
                    )
                , detailCard "Workflow Progress"
                    Nothing
                    (workflowView dealStageOptions d.stage)
                , detailCard "Change stage"
                    Nothing
                    (dealStagePillsDetail isMoving d)
                , detailCard "Deal info"
                    Nothing
                    (div [ class "info-list" ]
                        [ infoRow iconCalendar "Close date" closeDateDisplay
                        , StaffPicker.row iconUserTiny "Owner" d.owner
                        , infoRow iconCalendar "Created" createdDisplay
                        , infoRow iconDeals "Currency" d.currency
                        ]
                    )
                , detailCard "Notes"
                    Nothing
                    (if String.isEmpty d.notes then
                        p [ class "detail-muted" ]
                            [ text "No notes yet. Record what matters about this deal." ]

                     else
                        p [ class "detail-notes" ] [ text d.notes ]
                    )
                ]
            , div [ class "detail__main" ]
                [ detailCard "Contact activity"
                    (case linkedContact of
                        Just contact ->
                            Just
                                (button
                                    [ class "detail-card__action"
                                    , type_ "button"
                                    , onClick (OpenedContactDetail contact)
                                    ]
                                    [ text "Open contact activity" ]
                                )

                        Nothing ->
                            Nothing
                    )
                    (detailEmpty
                        iconTasks
                        (case linkedContact of
                            Just contact ->
                                "Activity is logged on the linked contact"

                            Nothing ->
                                "No contact linked"
                        )
                        (case linkedContact of
                            Just contact ->
                                "Open " ++ contact.name ++ " to review or log calls, emails, meetings, and notes."

                            Nothing ->
                                "Edit this deal to link a contact before logging related activity."
                        )
                    )
                ]
            ]
        , div [ class "bottom-row" ]
            [ alertsPanel (dealAlerts model d)
            , activityPanel (dealActivity model d)
            ]
        ]
