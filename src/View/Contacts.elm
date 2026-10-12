module View.Contacts exposing (contactById, contactList, contactsView, stageBadge)

{-| Contacts list page.
-}

import Update.Access
import Html exposing (..)
import Html.Attributes as Attr exposing (class, disabled, id, placeholder, type_, value)
import Html.Events exposing (onClick, onInput)
import Json.Decode as D
import Router exposing (routeToPath)
import Svg
import Types exposing (..)
import View.Helpers exposing (initials, paginationBar, svgIcon, svgPath)
import View.Icons exposing (iconEdit, iconTrash)


stageBadge : String -> Html Msg
stageBadge stage =
    let
        cls =
            case String.toLower stage of
                "customer" ->
                    "badge badge--customer"

                "qualified" ->
                    "badge badge--qualified"

                "proposal" ->
                    "badge badge--proposal"

                "lead" ->
                    "badge badge--lead"

                "negotiation" ->
                    "badge badge--negotiation"

                "won" ->
                    "badge badge--won"

                "lost" ->
                    "badge badge--lost"

                _ ->
                    "badge"
    in
    span [ class cls ] [ text stage ]


contactRow : Contact -> Html Msg
contactRow c =
    tr [ class "contact-row" ]
        [ td []
            [ div [ class "contact-name-cell" ]
                [ node "crm-record-photo"
                    [ class "contact-avatar"
                    , Attr.attribute "entity" "contacts"
                    , Attr.attribute "record-id" c.id
                    , Attr.attribute "initials" (initials c.name)
                    , Attr.attribute "person-name" c.name
                    ]
                    []
                , div [ class "contact-name-info" ]
                    [ a [ class "contact-name record-link", Attr.href (routeToPath (ContactDetail c.id)) ] [ text c.name ]
                    , span [ class "contact-email" ] [ text c.email ]
                    ]
                ]
            ]
        , td [] [ text c.company ]
        , td [] [ stageBadge c.stage ]
        , td [ class "contact-date" ] [ text c.lastContact ]
        , td [ class "contact-actions-cell" ]
            [ button
                [ class "row-action"
                , type_ "button"
                , Attr.title "Edit"
                , Attr.attribute "aria-label" ("Edit " ++ c.name)
                , Attr.disabled (not c.editable)
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( OpenedEditContact c, True ))
                ]
                [ iconEdit ]
            , button
                [ class "row-action row-action--danger"
                , type_ "button"
                , Attr.title "Delete"
                , Attr.attribute "aria-label" ("Delete " ++ c.name)
                , Attr.disabled (not c.editable)
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( RequestedDeleteContact c, True ))
                ]
                [ iconTrash ]
            ]
        ]


contactsSkeleton : Html Msg
contactsSkeleton =
    div []
        [ div [ class "page-toolbar" ]
            [ div [ class "page-toolbar__search" ]
                [ input [ type_ "text", placeholder "Search contacts…", disabled True ] [] ]
            ]
        , div [ class "table-wrap" ]
            (List.repeat 6
                (div [ class "skeleton-row" ]
                    [ div [ class "skeleton-avatar" ] []
                    , div [ class "skeleton-line skeleton-line--medium" ] []
                    , div [ class "skeleton-line skeleton-line--short" ] []
                    ]
                )
            )
        ]


contactsView : Model -> Html Msg
contactsView model =
    case model.contacts of
        NotAsked ->
            contactsSkeleton

        Loading ->
            contactsSkeleton

        Failure msg ->
            div [ class "content__empty-block" ]
                [ text ("Could not load contacts: " ++ msg), button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick (NavigatedTo model.route) ] [ text "Retry" ] ]

        Success data ->
            let
                filtered =
                    data.items

                isQueryEmpty =
                    String.isEmpty (String.trim data.query)
            in
            div []
                [ div [ class "page-toolbar" ]
                    [ div [ class "page-toolbar__search" ]
                        [ svgIcon
                            [ Attr.attribute "viewBox" "0 0 24 24"
                            , Attr.attribute "width" "16"
                            , Attr.attribute "height" "16"
                            , Attr.attribute "fill" "none"
                            , Attr.attribute "stroke" "currentColor"
                            , Attr.attribute "stroke-width" "1.8"
                            , Attr.attribute "stroke-linecap" "round"
                            , Attr.attribute "stroke-linejoin" "round"
                            ]
                            [ Svg.node "circle"
                                [ Attr.attribute "cx" "11"
                                , Attr.attribute "cy" "11"
                                , Attr.attribute "r" "8"
                                ]
                                []
                            , svgPath "M21 21l-4.35-4.35"
                            ]
                        , input
                            [ type_ "search"
                            , Attr.attribute "aria-label" "Search contacts"
                            , placeholder "Search contacts…"
                            , value data.query
                            , onInput UpdatedContactsQuery
                            ]
                            []
                        ]
                    , button
                        [ class "ecc-btn ecc-btn--inline"
                        , type_ "button"
                        , Attr.disabled (not (Update.Access.canCreate model))
                        , onClick OpenedAddContact
                        ]
                        [ text "Add contact" ]
                    ]
                , if List.isEmpty filtered then
                    div [ class "empty-state" ]
                        [ div [ class "empty-state__icon" ]
                            [ svgIcon
                                [ Attr.attribute "viewBox" "0 0 24 24"
                                , Attr.attribute "width" "22"
                                , Attr.attribute "height" "22"
                                , Attr.attribute "fill" "none"
                                , Attr.attribute "stroke" "currentColor"
                                , Attr.attribute "stroke-width" "1.8"
                                , Attr.attribute "stroke-linecap" "round"
                                , Attr.attribute "stroke-linejoin" "round"
                                ]
                                [ svgPath "M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"
                                , Svg.node "circle"
                                    [ Attr.attribute "cx" "9"
                                    , Attr.attribute "cy" "7"
                                    , Attr.attribute "r" "4"
                                    ]
                                    []
                                , svgPath "M23 21v-2a4 4 0 0 0-3-3.87"
                                , svgPath "M16 3.13a4 4 0 0 1 0 7.75"
                                ]
                            ]
                        , h3 [ class "empty-state__title" ]
                            [ text
                                (if isQueryEmpty then
                                    "No contacts yet"

                                 else
                                    "No matches found"
                                )
                            ]
                        , p [ class "empty-state__desc" ]
                            [ text
                                (if isQueryEmpty then
                                    "Add your first contact to start building your CRM."

                                 else
                                    "Try a different search term or clear the filter."
                                )
                            ]
                        , if isQueryEmpty then
                            div [ class "empty-state__action" ]
                                [ button
                                    [ class "ecc-btn ecc-btn--inline"
                                    , type_ "button"
                                    , Attr.disabled (not (Update.Access.canCreate model))
                        , onClick OpenedAddContact
                                    ]
                                    [ text "Add your first contact" ]
                                ]

                          else
                            text ""
                        ]

                  else
                    div [ class "table-wrap" ]
                        [ table [ class "data-table" ]
                            [ thead []
                                [ tr []
                                    [ th [] [ text "Name" ]
                                    , th [] [ text "Company" ]
                                    , th [] [ text "Stage" ]
                                    , th [] [ text "Last contact" ]
                                    , th [ class "th-actions" ] [ text "" ]
                                    ]
                                ]
                            , tbody [] (List.map contactRow filtered)
                            ]
                        ]
                , paginationBar data.total data.offset data.limit ContactsPageChanged
                ]


contactList : Model -> List Contact
contactList model =
    case model.contacts of
        Success data ->
            data.items

        _ ->
            []


contactById : List Contact -> String -> Maybe Contact
contactById contacts id =
    if String.isEmpty id then
        Nothing

    else
        contacts
            |> List.filter (\c -> c.id == id)
            |> List.head
