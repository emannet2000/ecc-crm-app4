module View.ContactForm exposing (contactFormModal, deleteConfirmModal)

{-| Contact create/edit modal and delete confirmation.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class, disabled, for, id, placeholder, type_, value)
import Html.Events exposing (onClick, onInput, onSubmit)
import Json.Decode as D
import Types exposing (..)
import View.Helpers exposing (onEnter, svgIcon, svgPath)


stageOptions : List String
stageOptions =
    [ "Lead", "Qualified", "Proposal", "Customer" ]


contactFormFieldError : String -> ContactForm -> Maybe String
contactFormFieldError field cf =
    cf.errors
        |> List.filter (\( f, _ ) -> f == field)
        |> List.head
        |> Maybe.map Tuple.second


richField : ContactForm -> String -> String -> String -> Bool -> Html Msg
richField cf fieldId labelText inputType shouldFocus =
    let
        currentValue =
            case fieldId of
                "name" ->
                    cf.name

                "email" ->
                    cf.email

                "company" ->
                    cf.company

                "title" ->
                    cf.title

                "phone" ->
                    cf.phone

                "location" ->
                    cf.location

                _ ->
                    ""

        err =
            contactFormFieldError fieldId cf

        cls =
            case err of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"

        baseAttrs =
            [ id ("cf-" ++ fieldId)
            , type_ inputType
            , placeholder " "
            , value currentValue
            , onInput (UpdatedContactFormField fieldId)
            , disabled cf.submitting
            ]

        finalAttrs =
            if shouldFocus then
                baseAttrs ++ [ Attr.autofocus True ]

            else
                baseAttrs
    in
    div [ class cls ]
        ([ input finalAttrs []
         , label [ for ("cf-" ++ fieldId) ] [ text labelText ]
         ]
            ++ (case err of
                    Just msg ->
                        [ p [ class "ecc-field__message" ] [ text msg ] ]

                    Nothing ->
                        []
               )
        )


stagePills : ContactForm -> Html Msg
stagePills cf =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Stage" ]
        , div [ class "stage-pills" ]
            (List.map
                (\s ->
                    let
                        cls =
                            if cf.stage == s then
                                "stage-pill stage-pill--active"

                            else
                                "stage-pill"
                    in
                    button
                        [ type_ "button"
                        , class cls
                        , onClick (UpdatedContactFormField "stage" s)
                        , disabled cf.submitting
                        ]
                        [ text s ]
                )
                stageOptions
            )
        ]


tagsField : ContactForm -> Html Msg
tagsField cf =
    div [ class "ecc-field ecc-field--tags" ]
        [ span [ class "ecc-field__label" ] [ text "Tags" ]
        , div [ class "tag-input-row" ]
            [ input
                [ type_ "text"
                , id "cf-tagInput"
                , placeholder "Add a tag…"
                , value cf.tagInput
                , onInput (UpdatedContactFormField "tagInput")
                , onEnter AddedContactTag
                , disabled cf.submitting
                ]
                []
            , button
                [ type_ "button"
                , class "tag-add-btn"
                , onClick AddedContactTag
                , disabled cf.submitting
                ]
                [ text "Add" ]
            ]
        , if List.isEmpty cf.tags then
            text ""

          else
            div [ class "tag-chip-list" ]
                (List.map
                    (\t ->
                        span [ class "tag-chip" ]
                            [ text t
                            , button
                                [ type_ "button"
                                , class "tag-chip__remove"
                                , onClick (RemovedContactTag t)
                                , disabled cf.submitting
                                , Attr.attribute "aria-label" ("Remove tag " ++ t)
                                ]
                                [ text "×" ]
                            ]
                    )
                    cf.tags
                )
        ]


notesField : ContactForm -> Html Msg
notesField cf =
    div [ class "ecc-field ecc-field--notes" ]
        [ span [ class "ecc-field__label" ] [ text "Notes" ]
        , textarea
            [ id "cf-notes"
            , placeholder "Record what matters about this relationship…"
            , value cf.notes
            , onInput (UpdatedContactFormField "notes")
            , disabled cf.submitting
            , Attr.rows 4
            ]
            []
        ]


discardConfirmView : Html Msg
discardConfirmView =
    div [ class "modal__confirm" ]
        [ p [ class "modal__confirm-text" ]
            [ text "Discard your changes? They won't be saved." ]
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick CancelledCloseContactForm
                ]
                [ text "Keep editing" ]
            , button
                [ type_ "button"
                , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                , onClick ConfirmedCloseContactForm
                ]
                [ text "Discard" ]
            ]
        ]


contactFormView : ContactForm -> Bool -> Html Msg
contactFormView cf isEdit =
    let
        formError =
            contactFormFieldError "form" cf

        submitLabel =
            if cf.submitting then
                "Saving…"

            else if isEdit then
                "Save changes"

            else
                "Save contact"
    in
    form [ onSubmit SubmittedContactForm, Attr.novalidate True ]
        [ Html.node "crm-draft-recovery" [] []
        , case formError of
            Just msg ->
                div [ class "ecc-alert ecc-alert--error" ]
                    [ text msg ]

            Nothing ->
                text ""
        , div [ class "form-grid" ]
            [ richField cf "name" "Full name" "text" True
            , richField cf "email" "Email address" "email" False
            , richField cf "company" "Company" "text" False
            , richField cf "title" "Job title" "text" False
            , richField cf "phone" "Phone" "tel" False
            , richField cf "location" "Location" "text" False
            ]
        , stagePills cf
        , tagsField cf
        , notesField cf
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick RequestedCloseContactForm
                , disabled cf.submitting
                ]
                [ text "Cancel" ]
            , button
                [ type_ "submit"
                , class "ecc-btn ecc-btn--inline"
                , disabled (cf.submitting || not cf.dirty)
                ]
                [ text submitLabel ]
            ]
        ]


contactFormModal : Model -> ContactForm -> Html Msg
contactFormModal model cf =
    let
        isEdit =
            model.editingId /= Nothing

        titleText =
            if isEdit then
                "Edit contact"

            else
                "Add contact"
    in
    div [ class "modal-backdrop", onClick RequestedCloseContactForm ]
        [ div
            [ class "modal modal--wide"
            , Attr.attribute "role" "dialog"
            , Attr.attribute "data-draft-record-id" (Maybe.withDefault "new" model.editingId)
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" titleText
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( DismissedToast, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text titleText ]
                , button
                    [ class "modal__close"
                    , type_ "button"
                    , onClick RequestedCloseContactForm
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
            , if cf.confirmDiscard then
                discardConfirmView

              else
                contactFormView cf isEdit
            ]
        ]


deleteConfirmModal : Contact -> Html Msg
deleteConfirmModal contact =
    div [ class "modal-backdrop", onClick CancelledDeleteContact ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Delete contact"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( DismissedToast, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete contact" ]
                , button
                    [ class "modal__close"
                    , type_ "button"
                    , onClick CancelledDeleteContact
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
                , strong [] [ text contact.name ]
                , text "? This cannot be undone."
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledDeleteContact
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedDeleteContact
                    ]
                    [ text "Delete" ]
                ]
            ]
        ]
