module View.Activity exposing (activityFeed, activityFormModal, deleteActivityConfirmModal)

{-| Activity feed and activity form.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class, disabled, for, id, placeholder, type_, value)
import Html.Events exposing (onClick, onInput, onSubmit)
import Json.Decode as D
import Types exposing (..)
import View.Helpers exposing (detailEmpty, svgIcon, svgPath)
import View.Icons exposing (iconTasks, iconTrash)


activityKindLabel : String -> String
activityKindLabel kind =
    case kind of
        "email" ->
            "Email"

        "whatsapp" ->
            "WhatsApp"

        "call" ->
            "Call"

        "meeting" ->
            "Meeting"

        "note" ->
            "Note"

        "task" ->
            "Task"

        _ ->
            "Activity"


activityItemView : Activity -> Html Msg
activityItemView a =
    div [ class "activity-item" ]
        [ div [ class "activity-item__marker", class ("activity-item__marker--" ++ a.kind) ] []
        , div [ class "activity-item__body" ]
            [ div [ class "activity-item__meta" ]
                [ span [ class "activity-item__kind" ]
                    [ text (activityKindLabel a.kind) ]
                , span [ class "activity-item__time" ]
                    [ text a.occurredAt ]
                , if String.isEmpty a.createdBy then
                    text ""

                  else
                    span [ class "activity-item__by" ]
                        [ text ("· " ++ a.createdBy) ]
                ]
            , div [ class "activity-item__title-row" ]
                [ h4 [ class "activity-item__title" ] [ text a.title ]
                , button
                    [ class "row-action row-action--danger row-action--tiny"
                    , type_ "button"
                    , Attr.title "Delete activity"
                    , Attr.attribute "aria-label" ("Delete " ++ a.title)
                    , onClick (RequestedDeleteActivity a)
                    ]
                    [ iconTrash ]
                ]
            , if String.isEmpty a.body then
                text ""

              else
                p [ class "activity-item__text" ] [ text a.body ]
            ]
        ]


activityFeed : Model -> Html Msg
activityFeed model =
    case model.activities of
        NotAsked ->
            div [ class "activity-loading" ] [ text "Loading activity…" ]

        Loading ->
            div [ class "activity-loading" ] [ text "Loading activity…" ]

        Failure msg ->
            div [ class "activity-loading" ] [ text ("Could not load activity: " ++ msg), button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick (NavigatedTo model.route) ] [ text "Retry" ] ]

        Success [] ->
            detailEmpty
                iconTasks
                "No activity yet"
                "Log your first call, email, or meeting to keep the timeline up to date."

        Success items ->
            div [ class "activity-list" ]
                (List.map activityItemView items)


activityFormFieldError : String -> ActivityForm -> Maybe String
activityFormFieldError field af =
    af.errors
        |> List.filter (\( f, _ ) -> f == field)
        |> List.head
        |> Maybe.map Tuple.second


activityKindPills : ActivityForm -> Html Msg
activityKindPills af =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Kind" ]
        , div [ class "stage-pills" ]
            (List.map
                (\k ->
                    let
                        cls =
                            if af.kind == k then
                                "stage-pill stage-pill--active"

                            else
                                "stage-pill"
                    in
                    button
                        [ type_ "button"
                        , class cls
                        , onClick (UpdatedActivityFormField "kind" k)
                        , disabled af.submitting
                        ]
                        [ text (activityKindLabel k) ]
                )
                activityKinds
            )
        , case activityFormFieldError "kind" af of
            Just msg ->
                p [ class "ecc-field__message" ] [ text msg ]

            Nothing ->
                text ""
        ]


activityFormView : ActivityForm -> Html Msg
activityFormView af =
    let
        formError =
            activityFormFieldError "form" af

        titleError =
            activityFormFieldError "title" af

        titleCls =
            case titleError of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"

        submitLabel =
            if af.submitting then
                "Saving…"

            else
                "Log activity"
    in
    form [ onSubmit SubmittedActivityForm, Attr.novalidate True ]
        [ Html.node "crm-draft-recovery" [ Attr.attribute "record-id" "new" ] []
        , case formError of
            Just msg ->
                div [ class "ecc-alert ecc-alert--error" ] [ text msg ]

            Nothing ->
                text ""
        , activityKindPills af
        , div [ class titleCls ]
            [ input
                [ id "af-title"
                , type_ "text"
                , placeholder " "
                , value af.title
                , onInput (UpdatedActivityFormField "title")
                , disabled af.submitting
                , Attr.autofocus True
                ]
                []
            , label [ for "af-title" ] [ text "Title" ]
            , case titleError of
                Just msg ->
                    p [ class "ecc-field__message" ] [ text msg ]

                Nothing ->
                    text ""
            ]
        , div [ class "ecc-field" ]
            [ input
                [ id "af-occurredAt"
                , type_ "date"
                , placeholder " "
                , value af.occurredAt
                , onInput (UpdatedActivityFormField "occurredAt")
                , disabled af.submitting
                ]
                []
            , label [ for "af-occurredAt" ] [ text "Date" ]
            ]
        , div [ class "ecc-field ecc-field--notes" ]
            [ span [ class "ecc-field__label" ] [ text "Details" ]
            , textarea
                [ id "af-body"
                , placeholder "What happened?"
                , value af.body
                , onInput (UpdatedActivityFormField "body")
                , disabled af.submitting
                , Attr.rows 4
                ]
                []
            ]
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick RequestedCloseActivityForm
                , disabled af.submitting
                ]
                [ text "Cancel" ]
            , button
                [ type_ "submit"
                , class "ecc-btn ecc-btn--inline"
                , disabled af.submitting
                ]
                [ text submitLabel ]
            ]
        ]


discardActivityConfirmView : Html Msg
discardActivityConfirmView =
    div [ class "modal__confirm" ]
        [ p [ class "modal__confirm-text" ]
            [ text "Discard your changes? They won't be saved." ]
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick CancelledCloseActivityForm
                ]
                [ text "Keep editing" ]
            , button
                [ type_ "button"
                , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                , onClick ConfirmedCloseActivityForm
                ]
                [ text "Discard" ]
            ]
        ]


activityFormModal : ActivityForm -> Html Msg
activityFormModal af =
    div [ class "modal-backdrop", onClick RequestedCloseActivityForm ]
        [ div
            [ class "modal modal--wide"
            , Attr.attribute "role" "dialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Log activity"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( NoOp, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Log activity" ]
                , button
                    [ class "modal__close"
                    , type_ "button"
                    , onClick RequestedCloseActivityForm
                    , Attr.attribute "aria-label" "Close"
                    ]
                    [ svgIcon
                        [ Attr.attribute "viewBox" "0 0 24 24"
                        , Attr.attribute "width" "18"
                        , Attr.attribute "height" "18"
                        , Attr.attribute "fill" "none"
                        , Attr.attribute "stroke" "currentColor"
                        , Attr.attribute "stroke-width" "2"
                        , Attr.attribute "stroke-linecap" "round"
                        , Attr.attribute "stroke-linejoin" "round"
                        ]
                        [ svgPath "M18 6 6 18"
                        , svgPath "M6 6l12 12"
                        ]
                    ]
                ]
            , if af.confirmDiscard then
                discardActivityConfirmView

              else
                activityFormView af
            ]
        ]


deleteActivityConfirmModal : Activity -> Html Msg
deleteActivityConfirmModal a =
    div [ class "modal-backdrop", onClick CancelledDeleteActivity ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Delete activity"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( NoOp, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete activity" ]
                , button
                    [ class "modal__close"
                    , type_ "button"
                    , onClick CancelledDeleteActivity
                    , Attr.attribute "aria-label" "Close"
                    ]
                    [ svgIcon
                        [ Attr.attribute "viewBox" "0 0 24 24"
                        , Attr.attribute "width" "18"
                        , Attr.attribute "height" "18"
                        , Attr.attribute "fill" "none"
                        , Attr.attribute "stroke" "currentColor"
                        , Attr.attribute "stroke-width" "2"
                        , Attr.attribute "stroke-linecap" "round"
                        , Attr.attribute "stroke-linejoin" "round"
                        ]
                        [ svgPath "M18 6 6 18"
                        , svgPath "M6 6l12 12"
                        ]
                    ]
                ]
            , p [ class "modal__confirm-text" ]
                [ text "Delete "
                , strong [] [ text a.title ]
                , text "? This cannot be undone."
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledDeleteActivity
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedDeleteActivity
                    ]
                    [ text "Delete" ]
                ]
            ]
        ]
