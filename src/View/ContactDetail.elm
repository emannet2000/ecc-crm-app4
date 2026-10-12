module View.ContactDetail exposing (contactDetailView)

{-| Contact detail page.
-}

import Update.Access
import Html exposing (..)
import Html.Attributes as Attr exposing (class, type_)
import Html.Events exposing (onClick)
import Types exposing (..)
import View.Activity exposing (activityFeed)
import View.Contacts exposing (stageBadge)
import View.Format exposing (formatCurrencyWith)
import View.Helpers exposing (detailCard, detailEmpty, detailStat, infoRow, initials)
import View.Icons exposing (iconBack, iconCalendar, iconContacts, iconDeals, iconEdit, iconMail, iconPhone, iconPin, iconTasks, iconTrash)


openDealRow : Deal -> Html Msg
openDealRow d =
    div
        [ class "activity-item"
        , Attr.attribute "role" "button"
        , Attr.attribute "tabindex" "0"
        , onClick (OpenedDealDetail d)
        ]
        [ div [ class "activity-item__marker" ] []
        , div [ class "activity-item__body" ]
            [ div [ class "activity-item__title-row" ]
                [ h4 [ class "activity-item__title" ] [ text d.title ]
                , span [ class "badge badge--muted" ]
                    [ text (formatCurrencyWith d.currency d.value) ]
                ]
            , div [ class "activity-item__meta" ]
                [ span [ class "activity-item__kind" ] [ text d.stage ]
                , span [ class "activity-item__time" ]
                    [ text
                        (if String.isEmpty d.closeDate then
                            "No close date"

                         else
                            "Closes " ++ d.closeDate
                        )
                    ]
                ]
            ]
        ]


taskStatusLabel : String -> String
taskStatusLabel s =
    case s of
        "todo" ->
            "To do"

        "in_progress" ->
            "In progress"

        "done" ->
            "Done"

        _ ->
            s


taskStatusClass : String -> String
taskStatusClass s =
    case s of
        "todo" ->
            "badge badge--muted"

        "in_progress" ->
            "badge badge--info"

        "done" ->
            "badge badge--success"

        _ ->
            "badge"


taskRow : Task -> Html Msg
taskRow t =
    div [ class "activity-item" ]
        [ div [ class "activity-item__marker" ] []
        , div [ class "activity-item__body" ]
            [ div [ class "activity-item__title-row" ]
                [ h4 [ class "activity-item__title" ] [ text t.title ]
                , span [ class (taskStatusClass t.status) ]
                    [ text (taskStatusLabel t.status) ]
                ]
            , div [ class "activity-item__meta" ]
                [ span [ class "activity-item__kind" ]
                    [ text
                        (if String.isEmpty t.dueDate then
                            "No due date"

                         else
                            "Due " ++ t.dueDate
                        )
                    ]
                , if String.isEmpty t.owner then
                    text ""

                  else
                    span [ class "activity-item__by" ]
                        [ text ("· " ++ t.owner) ]
                ]
            ]
        ]


contactDetailView : Model -> Contact -> Html Msg
contactDetailView model c =
    let
        displayCompany =
            if String.isEmpty c.company then
                "—"

            else
                c.company

        displayTitle =
            if String.isEmpty c.title then
                ""

            else
                c.title

        displayLocation =
            if String.isEmpty c.location then
                "—"

            else
                c.location

        displayPhone =
            if String.isEmpty c.phone then
                "—"

            else
                c.phone

        displayOwner =
            if String.isEmpty c.owner then
                "Unassigned"

            else
                c.owner

        displayCreated =
            if String.isEmpty c.createdAt then
                "—"

            else
                c.createdAt

        roleLine =
            if String.isEmpty displayTitle then
                displayCompany

            else
                displayTitle ++ " · " ++ displayCompany

        activityCount =
            case model.activities of
                Success items ->
                    List.length items

                _ ->
                    0

        openDeals =
            case model.deals of
                Success d ->
                    List.filter
                        (\x -> x.contactId == c.id && x.stage /= "Won" && x.stage /= "Lost")
                        d.items

                _ ->
                    []

        tasksDue =
            case model.tasks of
                Success t ->
                    List.filter
                        (\x -> x.contactId == c.id && x.status /= "done")
                        t.items

                _ ->
                    []

        openDealsHint =
            if List.isEmpty openDeals then
                "No open deals"

            else
                "Active"

        tasksDueHint =
            if List.isEmpty tasksDue then
                "All clear"

            else
                "Pending"
    in
    div [ class "detail" ]
        [ button
            [ class "detail__back"
            , type_ "button"
            , onClick (NavigatedTo Contacts)
            ]
            [ iconBack
            , span [] [ text "Back to contacts" ]
            ]
        , header [ class "detail-hero" ]
            [ node "crm-record-photo"
                [ class "detail-hero__avatar"
                , Attr.attribute "entity" "contacts"
                , Attr.attribute "record-id" c.id
                , Attr.attribute "initials" (initials c.name)
                , Attr.attribute "editable" (if Maybe.withDefault False (Maybe.map (\u -> u.role /= "viewer") model.user) then "true" else "false")
                ]
                []
            , div [ class "detail-hero__body" ]
                [ div [ class "detail-hero__title-row" ]
                    [ h1 [ class "detail-hero__name" ] [ text c.name ]
                    , stageBadge c.stage
                    ]
                , p [ class "detail-hero__role" ] [ text roleLine ]
                , div [ class "detail-hero__contact" ]
                    [ a
                        [ class "detail-hero__chip"
                        , Attr.href ("mailto:" ++ c.email)
                        ]
                        [ iconMail
                        , span [] [ text c.email ]
                        ]
                    , if String.isEmpty c.phone then
                        text ""

                      else
                        a
                            [ class "detail-hero__chip"
                            , Attr.href ("tel:" ++ c.phone)
                            ]
                            [ iconPhone
                            , span [] [ text c.phone ]
                            ]
                    , if String.isEmpty c.location then
                        text ""

                      else
                        span [ class "detail-hero__chip" ]
                            [ iconPin
                            , span [] [ text c.location ]
                            ]
                    ]
                ]
            , div [ class "detail-hero__actions" ]
                [ button
                    [ class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , type_ "button"
                    , Attr.disabled (not c.editable)
            , onClick (OpenedEditContact c)
                    ]
                    [ iconEdit
                    , span [] [ text "Edit" ]
                    ]
                , button
                    [ class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , type_ "button"
                    , Attr.disabled (not c.editable)
            , onClick (RequestedDeleteContact c)
                    ]
                    [ iconTrash
                    , span [] [ text "Delete" ]
                    ]
                ]
            ]
        , div [ class "detail-stats" ]
            [ detailStat "Open deals"
                (String.fromInt (List.length openDeals))
                openDealsHint
            , detailStat "Activities"
                (String.fromInt activityCount)
                "Logged"
            , detailStat "Tasks due"
                (String.fromInt (List.length tasksDue))
                tasksDueHint
            , detailStat "Last contact" c.lastContact "Last touch"
            ]
        , div [ class "detail__grid" ]
            [ aside [ class "detail__sidebar" ]
                [ detailCard "About"
                    Nothing
                    (div [ class "info-list" ]
                        [ infoRow iconMail "Email" c.email
                        , infoRow iconPhone "Phone" displayPhone
                        , infoRow iconPin "Location" displayLocation
                        , infoRow iconCalendar "Created" displayCreated
                        , infoRow iconContacts "Owner" displayOwner
                        ]
                    )
                , detailCard "Tags"
                    Nothing
                    (if List.isEmpty c.tags then
                        p [ class "detail-muted" ] [ text "No tags yet." ]

                     else
                        div [ class "tag-list" ]
                            (List.map (\t -> span [ class "tag" ] [ text t ]) c.tags)
                    )
                , detailCard "Notes"
                    Nothing
                    (if String.isEmpty c.notes then
                        p [ class "detail-muted" ]
                            [ text "No notes yet. Record what matters about this relationship." ]

                     else
                        p [ class "detail-notes" ] [ text c.notes ]
                    )
                ]
            , div [ class "detail__main" ]
                [ detailCard "Activity"
                    (Just
                        (button
                            [ class "detail-card__action"
                            , type_ "button"
                            , Attr.disabled (not c.editable)
                            , onClick OpenedActivityForm
                            ]
                            [ text "Log activity" ]
                        )
                    )
                    (activityFeed model)
                , detailCard "Open deals"
                    (Just
                        (button
                            [ class "detail-card__action"
                            , type_ "button"
                            , Attr.disabled (not (Update.Access.canCreate model))
                        , onClick (OpenedAddDealForContact c)
                            ]
                            [ text "New deal" ]
                        )
                    )
                    (if List.isEmpty openDeals then
                        detailEmpty
                            iconDeals
                            "No open deals"
                            "Link this contact to a deal to see pipeline value and stage."

                     else
                        div [ class "activity-list" ]
                            (List.map openDealRow openDeals)
                    )
                , detailCard "Tasks"
                    (Just
                        (button
                            [ class "detail-card__action"
                            , type_ "button"
                            , Attr.disabled (not (Update.Access.canCreate model))
                        , onClick (OpenedAddTaskForContact c)
                            ]
                            [ text "New task" ]
                        )
                    )
                    (if List.isEmpty tasksDue then
                        detailEmpty
                            iconTasks
                            "No open tasks"
                            "Add a follow-up to make sure nothing slips through the cracks."

                     else
                        div [ class "activity-list" ]
                            (List.map taskRow tasksDue)
                    )
                ]
            ]
        ]
