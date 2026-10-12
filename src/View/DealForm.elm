module View.DealForm exposing (bulkDeleteConfirmModal, bulkMoveConfirmModal, contactsForSelect, dealFormModal, deleteDealConfirmModal)

{-| Deal create/edit form, confirmations, bulk actions.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class, disabled, for, id, placeholder, type_, value)
import Html.Events exposing (onClick, onInput, onSubmit)
import Json.Decode as D
import Types exposing (..)
import View.StaffPicker as StaffPicker
import View.Deals exposing (dealStageOptions)
import View.Helpers exposing (svgIcon, svgPath)


dealFormFieldError : String -> DealForm -> Maybe String
dealFormFieldError field df =
    df.errors
        |> List.filter (\( f, _ ) -> f == field)
        |> List.head
        |> Maybe.map Tuple.second


contactsForSelect : Model -> List Contact
contactsForSelect model =
    let
        loaded =
            case model.contacts of
                Success data ->
                    data.items

                _ ->
                    []

        current =
            case model.viewingContact of
                Just contact ->
                    if List.any (\item -> item.id == contact.id) loaded then
                        []

                    else
                        [ contact ]

                Nothing ->
                    []
    in
    List.sortBy .name (loaded ++ current)


dealContactSelect : Model -> DealForm -> Html Msg
dealContactSelect model df =
    let
        contactOpts =
            contactsForSelect model

        err =
            dealFormFieldError "contactId" df

        cls =
            case err of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"
    in
    div [ class cls ]
        ([ select
            [ id "df-contactId"
            , onInput (UpdatedDealFormField "contactId")
            , disabled df.submitting
            ]
            (option [ value "", Attr.selected (df.contactId == "") ]
                [ text "No contact linked" ]
                :: List.map
                    (\c ->
                        option
                            [ value c.id, Attr.selected (df.contactId == c.id) ]
                            [ text c.name ]
                    )
                    contactOpts
            )
         , label [ for "df-contactId" ] [ text "Contact" ]
         ]
            ++ (case err of
                    Just msg ->
                        [ p [ class "ecc-field__message" ] [ text msg ] ]

                    Nothing ->
                        []
               )
        )


dealCurrencySelect : DealForm -> Html Msg
dealCurrencySelect df =
    div [ class "ecc-field" ]
        [ select
            [ id "df-currency"
            , onInput (UpdatedDealFormField "currency")
            ]
            (List.map
                (\c ->
                    option
                        [ value c
                        , Attr.selected (df.currency == c)
                        ]
                        [ text c ]
                )
                [ "USD", "EUR", "GBP", "AED", "AUD", "CAD", "PHP", "INR", "SGD", "JPY" ]
            )
        , label [ for "df-currency" ] [ text "Currency" ]
        ]


dealRichField : DealForm -> String -> String -> String -> Bool -> Html Msg
dealRichField df fieldId labelText inputType shouldFocus =
    let
        currentValue =
            case fieldId of
                "title" ->
                    df.title

                "value" ->
                    df.value

                "currency" ->
                    df.currency

                "closeDate" ->
                    df.closeDate

                "owner" ->
                    df.owner

                _ ->
                    ""

        err =
            dealFormFieldError fieldId df

        cls =
            case err of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"

        baseAttrs =
            [ id ("df-" ++ fieldId)
            , type_ inputType
            , placeholder " "
            , value currentValue
            , onInput (UpdatedDealFormField fieldId)
            , disabled df.submitting
            ]

        finalAttrs =
            if shouldFocus then
                baseAttrs ++ [ Attr.autofocus True ]

            else
                baseAttrs
    in
    div [ class cls ]
        ([ input finalAttrs []
         , label [ for ("df-" ++ fieldId) ] [ text labelText ]
         ]
            ++ (case err of
                    Just msg ->
                        [ p [ class "ecc-field__message" ] [ text msg ] ]

                    Nothing ->
                        []
               )
        )


dealStagePills : DealForm -> Html Msg
dealStagePills df =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Stage" ]
        , div [ class "stage-pills" ]
            (List.map
                (\s ->
                    let
                        cls =
                            if df.stage == s then
                                "stage-pill stage-pill--active"

                            else
                                "stage-pill"
                    in
                    button
                        [ type_ "button"
                        , class cls
                        , onClick (UpdatedDealFormField "stage" s)
                        , disabled df.submitting
                        ]
                        [ text s ]
                )
                dealStageOptions
            )
        ]


dealNotesField : DealForm -> Html Msg
dealNotesField df =
    div [ class "ecc-field ecc-field--notes" ]
        [ span [ class "ecc-field__label" ] [ text "Notes" ]
        , textarea
            [ id "df-notes"
            , placeholder "Add context about this deal…"
            , value df.notes
            , onInput (UpdatedDealFormField "notes")
            , disabled df.submitting
            , Attr.rows 4
            ]
            []
        ]


discardDealConfirmView : Html Msg
discardDealConfirmView =
    div [ class "modal__confirm" ]
        [ p [ class "modal__confirm-text" ]
            [ text "Discard your changes? They won't be saved." ]
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick CancelledCloseDealForm
                ]
                [ text "Keep editing" ]
            , button
                [ type_ "button"
                , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                , onClick ConfirmedCloseDealForm
                ]
                [ text "Discard" ]
            ]
        ]


dealFormView : Model -> DealForm -> Bool -> Html Msg
dealFormView model df isEdit =
    let
        formError =
            dealFormFieldError "form" df

        submitLabel =
            if df.submitting then
                "Saving…"

            else if isEdit then
                "Save changes"

            else
                "Save deal"
    in
    form [ onSubmit SubmittedDealForm, Attr.novalidate True ]
        [ Html.node "crm-draft-recovery" [] []
        , case formError of
            Just msg ->
                div [ class "ecc-alert ecc-alert--error" ] [ text msg ]

            Nothing ->
                text ""
        , div [ class "form-grid" ]
            [ dealRichField df "title" "Deal title" "text" True
            , dealContactSelect model df
            , dealRichField df "value" "Value" "number" False
            , dealCurrencySelect df
            , dealRichField df "closeDate" "Expected close" "date" False
            , div [] [ StaffPicker.view "df-owner" "deals" model.editingDealId df.owner "Owner" df.submitting (UpdatedDealFormField "owner"), p [ class "ecc-field__message" ] [ text (Maybe.withDefault "" (dealFormFieldError "owner" df)) ] ]
            ]
        , dealStagePills df
        , dealNotesField df
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick RequestedCloseDealForm
                , disabled df.submitting
                ]
                [ text "Cancel" ]
            , button
                [ type_ "submit"
                , class "ecc-btn ecc-btn--inline"
                , disabled (df.submitting || not df.dirty)
                ]
                [ text submitLabel ]
            ]
        ]


dealFormModal : Model -> DealForm -> Html Msg
dealFormModal model df =
    let
        isEdit =
            model.editingDealId /= Nothing

        titleText =
            if isEdit then
                "Edit deal"

            else
                "Add deal"
    in
    div [ class "modal-backdrop", onClick RequestedCloseDealForm ]
        [ div
            [ class "modal modal--wide"
            , Attr.attribute "role" "dialog"
            , Attr.attribute "data-draft-record-id" (Maybe.withDefault "new" model.editingDealId)
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" titleText
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( NoOp, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text titleText ]
                , button
                    [ class "modal__close"
                    , type_ "button"
                    , onClick RequestedCloseDealForm
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
            , if df.confirmDiscard then
                discardDealConfirmView

              else
                dealFormView model df isEdit
            ]
        ]


deleteDealConfirmModal : Deal -> Html Msg
deleteDealConfirmModal deal =
    div [ class "modal-backdrop", onClick CancelledDeleteDeal ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Delete deal"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( NoOp, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete deal" ]
                , button
                    [ class "modal__close"
                    , type_ "button"
                    , onClick CancelledDeleteDeal
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
                , strong [] [ text deal.title ]
                , text "? This cannot be undone."
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledDeleteDeal
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedDeleteDeal
                    ]
                    [ text "Delete" ]
                ]
            ]
        ]


bulkDeleteConfirmModal : Int -> Html Msg
bulkDeleteConfirmModal n =
    div [ class "modal-backdrop", onClick CancelledBulkDelete ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Delete selected deals"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( NoOp, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete selected deals" ]
                ]
            , p [ class "modal__confirm-text" ]
                [ text "Delete "
                , strong [] [ text (String.fromInt n) ]
                , text " deal"
                , text
                    (if n == 1 then
                        ""

                     else
                        "s"
                    )
                , text "? This cannot be undone."
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledBulkDelete
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedBulkDelete
                    ]
                    [ text "Delete all" ]
                ]
            ]
        ]


bulkMoveConfirmModal : String -> Int -> Html Msg
bulkMoveConfirmModal currentStage n =
    div [ class "modal-backdrop", onClick CancelledBulkMove ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Move selected deals"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( NoOp, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Move selected deals" ]
                ]
            , div [ class "modal__confirm" ]
                [ p [ class "modal__confirm-text" ]
                    [ text "Move "
                    , strong [] [ text (String.fromInt n) ]
                    , text " deal"
                    , text
                        (if n == 1 then
                            ""

                         else
                            "s"
                        )
                    , text " to:"
                    ]
                , div [ class "stage-pills" ]
                    (List.map
                        (\s ->
                            button
                                [ type_ "button"
                                , class
                                    (if currentStage == s then
                                        "stage-pill stage-pill--active"

                                     else
                                        "stage-pill"
                                    )
                                , onClick (RequestedBulkMove s)
                                ]
                                [ text s ]
                        )
                        dealStageOptions
                    )
                , div [ class "modal__actions" ]
                    [ button
                        [ type_ "button"
                        , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                        , onClick CancelledBulkMove
                        ]
                        [ text "Cancel" ]
                    , button
                        [ type_ "button"
                        , class "ecc-btn ecc-btn--inline"
                        , onClick ConfirmedBulkMove
                        , disabled (String.isEmpty currentStage)
                        ]
                        [ text "Move" ]
                    ]
                ]
            ]
        ]
