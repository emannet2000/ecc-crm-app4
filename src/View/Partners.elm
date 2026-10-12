module View.Partners exposing (deletePartnerConfirmModal, partnerDetailView, partnerFormModal, partnersView)

{-| Partners: list, detail, form.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class, disabled, for, id, placeholder, type_, value)
import Html.Events exposing (onClick, onInput, onSubmit)
import Json.Decode as D
import Types exposing (..)
import View.Format exposing (formatCurrency)
import View.Helpers exposing (detailCard, detailStat, infoRow, initials, paginationBar, svgIcon, svgPath)
import View.Icons exposing (iconBack, iconCalendar, iconContacts, iconDeals, iconEdit, iconMail, iconPhone, iconTrash, iconUserTiny)


partnerTypeBadge : String -> Html Msg
partnerTypeBadge t =
    span [ class "badge badge--info" ] [ text t ]


partnerRow : Partner -> Html Msg
partnerRow partner =
    tr [ class "contact-row", onClick (OpenedPartnerDetail partner) ]
        [ td []
            [ div [ class "contact-name-cell" ]
                [ div [ class "contact-avatar" ] [ text (initials partner.legalCompanyName) ]
                , div [ class "contact-name-info" ]
                    [ span [ class "contact-name" ] [ text partner.legalCompanyName ]
                    , span [ class "contact-email" ]
                        [ text
                            (if String.isEmpty partner.contactPerson then
                                partner.country

                             else
                                partner.contactPerson
                            )
                        ]
                    ]
                ]
            ]
        , td [] [ partnerTypeBadge partner.type_ ]
        , td [] [ text partner.country ]
        , td [] [ text partner.licenseNumber ]
        , td [] [ text (String.fromInt partner.casesReferred) ]
        , td [] [ text (String.fromInt partner.casesConverted) ]
        , td [ class "contact-actions-cell" ]
            [ button
                [ class "row-action"
                , type_ "button"
                , Attr.title "Edit"
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( OpenedEditPartner partner, True ))
                ]
                [ iconEdit ]
            , button
                [ class "row-action row-action--danger"
                , type_ "button"
                , Attr.title "Delete"
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( RequestedDeletePartner partner, True ))
                ]
                [ iconTrash ]
            ]
        ]


partnersSkeleton : Html Msg
partnersSkeleton =
    div []
        [ div [ class "page-toolbar" ]
            [ div [ class "page-toolbar__search" ]
                [ input [ type_ "text", placeholder "Search partners…", disabled True ] [] ]
            ]
        , div [ class "table-wrap" ]
            (List.repeat 5
                (div [ class "skeleton-row" ]
                    [ div [ class "skeleton-line skeleton-line--medium" ] []
                    , div [ class "skeleton-line skeleton-line--short" ] []
                    ]
                )
            )
        ]


partnersView : Model -> Html Msg
partnersView model =
    case model.partners of
        NotAsked ->
            partnersSkeleton

        Loading ->
            partnersSkeleton

        Failure msg ->
            div [ class "content__empty-block" ]
                [ text ("Could not load partners: " ++ msg), button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick (NavigatedTo model.route) ] [ text "Retry" ] ]

        Success data ->
            let
                isQueryEmpty =
                    String.isEmpty (String.trim data.query)
            in
            div []
                [ div [ class "page-toolbar" ]
                    [ div [ class "page-toolbar__search" ]
                        [ input
                            [ type_ "search"
                            , Attr.attribute "aria-label" "Search partners"
                            , placeholder "Search partners…"
                            , value data.query
                            , onInput UpdatedPartnersQuery
                            ]
                            []
                        ]
                    , button
                        [ class "ecc-btn ecc-btn--inline"
                        , type_ "button"
                        , onClick OpenedAddPartner
                        ]
                        [ text "Add partner" ]
                    ]
                , div [ class "stage-pills", Attr.style "margin-bottom" "12px" ]
                    (button
                        [ type_ "button"
                        , class
                            (if data.typeFilter == "" then
                                "stage-pill stage-pill--active"

                             else
                                "stage-pill"
                            )
                        , onClick (UpdatedPartnersTypeFilter "")
                        ]
                        [ text "All types" ]
                        :: List.map
                            (\t ->
                                button
                                    [ type_ "button"
                                    , class
                                        (if data.typeFilter == t then
                                            "stage-pill stage-pill--active"

                                         else
                                            "stage-pill"
                                        )
                                    , onClick (UpdatedPartnersTypeFilter t)
                                    ]
                                    [ text t ]
                            )
                            partnerTypes
                    )
                , if List.isEmpty data.items then
                    div [ class "empty-state" ]
                        [ h3 [ class "empty-state__title" ]
                            [ text
                                (if isQueryEmpty then
                                    "No partners yet"

                                 else
                                    "No partners match"
                                )
                            ]
                        , p [ class "empty-state__desc" ]
                            [ text "Add partners to track agreements, licenses, and referrals." ]
                        ]

                  else
                    div [ class "table-wrap" ]
                        [ table [ class "data-table" ]
                            [ thead []
                                [ tr []
                                    [ th [] [ text "Company" ]
                                    , th [] [ text "Type" ]
                                    , th [] [ text "Country" ]
                                    , th [] [ text "License" ]
                                    , th [] [ text "Referred" ]
                                    , th [] [ text "Converted" ]
                                    , th [ class "th-actions" ] [ text "" ]
                                    ]
                                ]
                            , tbody [] (List.map partnerRow data.items)
                            ]
                        ]
                , paginationBar data.total data.offset data.limit PartnersPageChanged
                ]


partnerDetailView : Model -> Partner -> Html Msg
partnerDetailView _ partner =
    let
        display v =
            if String.isEmpty v then
                "—"

            else
                v
    in
    div [ class "detail" ]
        [ button
            [ class "detail__back"
            , type_ "button"
            , onClick (NavigatedTo Partners)
            ]
            [ iconBack
            , span [] [ text "Back to partners" ]
            ]
        , header [ class "detail-hero" ]
            [ div [ class "detail-hero__avatar" ] [ text (initials partner.legalCompanyName) ]
            , div [ class "detail-hero__body" ]
                [ div [ class "detail-hero__title-row" ]
                    [ h1 [ class "detail-hero__name" ] [ text partner.legalCompanyName ]
                    , partnerTypeBadge partner.type_
                    ]
                , p [ class "detail-hero__role" ] [ text (display partner.country) ]
                ]
            , div [ class "detail-hero__actions" ]
                [ button
                    [ class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , type_ "button"
                    , onClick (OpenedEditPartner partner)
                    ]
                    [ iconEdit
                    , span [] [ text "Edit" ]
                    ]
                , button
                    [ class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , type_ "button"
                    , onClick (RequestedDeletePartner partner)
                    ]
                    [ iconTrash
                    , span [] [ text "Delete" ]
                    ]
                ]
            ]
        , div [ class "detail-stats" ]
            [ detailStat "Cases referred" (String.fromInt partner.casesReferred) "Total"
            , detailStat "Converted" (String.fromInt partner.casesConverted) "Won"
            , detailStat "Amount payable" (formatCurrency partner.amountPayable) "Outstanding"
            , detailStat "Country" (display partner.country) "Location"
            ]
        , div [ class "detail__grid" ]
            [ aside [ class "detail__sidebar" ]
                [ detailCard "Contact"
                    Nothing
                    (div [ class "info-list" ]
                        [ infoRow iconUserTiny "Person" (display partner.contactPerson)
                        , infoRow iconMail "Email" (display partner.contactEmail)
                        , infoRow iconPhone "Phone" (display partner.contactPhone)
                        ]
                    )
                , detailCard "License"
                    Nothing
                    (div [ class "info-list" ]
                        [ infoRow iconDeals "License #" (display partner.licenseNumber)
                        , infoRow iconCalendar "Expires" (display partner.licenseExpiry)
                        , infoRow iconContacts "Verified by" (display partner.verificationSource)
                        ]
                    )
                , detailCard "Agreement"
                    Nothing
                    (div [ class "info-list" ]
                        [ infoRow iconCalendar "Start" (display partner.agreementStart)
                        , infoRow iconCalendar "Expiry" (display partner.agreementExpiry)
                        , infoRow iconDeals "Services" (display partner.servicesPermitted)
                        , infoRow iconDeals "Commission" (display partner.commissionStructure)
                        , infoRow iconDeals "Payment terms" (display partner.paymentTerms)
                        ]
                    )
                , detailCard "Notes"
                    Nothing
                    (if String.isEmpty partner.notes then
                        p [ class "detail-muted" ] [ text "No notes yet." ]

                     else
                        p [ class "detail-notes" ] [ text partner.notes ]
                    )
                ]
            , div [ class "detail__main" ]
                [ detailCard "Compliance"
                    Nothing
                    (if String.isEmpty partner.complianceNotes then
                        p [ class "detail-muted" ] [ text "No compliance notes on file." ]

                     else
                        p [ class "detail-notes" ] [ text partner.complianceNotes ]
                    )
                ]
            ]
        ]


partnerFormFieldError : String -> PartnerForm -> Maybe String
partnerFormFieldError field pf =
    pf.errors
        |> List.filter (\( f, _ ) -> f == field)
        |> List.head
        |> Maybe.map Tuple.second


partnerRichField : PartnerForm -> String -> String -> String -> Html Msg
partnerRichField pf fieldId labelText inputType =
    let
        currentValue =
            case fieldId of
                "legalCompanyName" ->
                    pf.legalCompanyName

                "country" ->
                    pf.country

                "licenseNumber" ->
                    pf.licenseNumber

                "licenseExpiry" ->
                    pf.licenseExpiry

                "verificationSource" ->
                    pf.verificationSource

                "contactPerson" ->
                    pf.contactPerson

                "contactEmail" ->
                    pf.contactEmail

                "contactPhone" ->
                    pf.contactPhone

                "agreementStart" ->
                    pf.agreementStart

                "agreementExpiry" ->
                    pf.agreementExpiry

                "servicesPermitted" ->
                    pf.servicesPermitted

                "commissionStructure" ->
                    pf.commissionStructure

                "paymentTerms" ->
                    pf.paymentTerms

                "casesReferred" ->
                    pf.casesReferred

                "casesConverted" ->
                    pf.casesConverted

                "amountPayable" ->
                    pf.amountPayable

                _ ->
                    ""

        err =
            partnerFormFieldError fieldId pf

        cls =
            case err of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"
    in
    div [ class cls ]
        ([ input
            [ id ("pf-" ++ fieldId)
            , type_ inputType
            , placeholder " "
            , value currentValue
            , onInput (UpdatedPartnerFormField fieldId)
            , disabled pf.submitting
            ]
            []
         , label [ for ("pf-" ++ fieldId) ] [ text labelText ]
         ]
            ++ (case err of
                    Just msg ->
                        [ p [ class "ecc-field__message" ] [ text msg ] ]

                    Nothing ->
                        []
               )
        )


partnerTypePills : PartnerForm -> Html Msg
partnerTypePills pf =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Partner type" ]
        , div [ class "stage-pills" ]
            (List.map
                (\t ->
                    button
                        [ type_ "button"
                        , class
                            (if pf.type_ == t then
                                "stage-pill stage-pill--active"

                             else
                                "stage-pill"
                            )
                        , onClick (UpdatedPartnerFormField "type" t)
                        , disabled pf.submitting
                        ]
                        [ text t ]
                )
                partnerTypes
            )
        ]


partnerFormView : PartnerForm -> Bool -> Html Msg
partnerFormView pf isEdit =
    let
        formError =
            partnerFormFieldError "form" pf

        submitLabel =
            if pf.submitting then
                "Saving…"

            else if isEdit then
                "Save changes"

            else
                "Save partner"
    in
    form [ onSubmit SubmittedPartnerForm, Attr.novalidate True ]
        [ Html.node "crm-draft-recovery" [] []
        , case formError of
            Just msg ->
                div [ class "ecc-alert ecc-alert--error" ] [ text msg ]

            Nothing ->
                text ""
        , div [ class "form-grid" ]
            [ partnerRichField pf "legalCompanyName" "Legal company name" "text"
            , partnerRichField pf "country" "Country (e.g. AE)" "text"
            , partnerRichField pf "licenseNumber" "License number" "text"
            , partnerRichField pf "licenseExpiry" "License expiry" "date"
            , partnerRichField pf "verificationSource" "Verification source" "text"
            , partnerRichField pf "contactPerson" "Contact person" "text"
            , partnerRichField pf "contactEmail" "Contact email" "email"
            , partnerRichField pf "contactPhone" "Contact phone" "tel"
            , partnerRichField pf "agreementStart" "Agreement start" "date"
            , partnerRichField pf "agreementExpiry" "Agreement expiry" "date"
            , partnerRichField pf "servicesPermitted" "Services permitted" "text"
            , partnerRichField pf "commissionStructure" "Commission structure" "text"
            , partnerRichField pf "paymentTerms" "Payment terms" "text"
            , partnerRichField pf "casesReferred" "Cases referred" "number"
            , partnerRichField pf "casesConverted" "Cases converted" "number"
            , partnerRichField pf "amountPayable" "Amount payable" "number"
            ]
        , partnerTypePills pf
        , div [ class "ecc-field ecc-field--notes" ]
            [ span [ class "ecc-field__label" ] [ text "Compliance notes" ]
            , textarea
                [ value pf.complianceNotes
                , onInput (UpdatedPartnerFormField "complianceNotes")
                , disabled pf.submitting
                , Attr.rows 2
                ]
                []
            ]
        , div [ class "ecc-field ecc-field--notes" ]
            [ span [ class "ecc-field__label" ] [ text "Notes" ]
            , textarea
                [ value pf.notes
                , onInput (UpdatedPartnerFormField "notes")
                , disabled pf.submitting
                , Attr.rows 2
                ]
                []
            ]
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick RequestedClosePartnerForm
                , disabled pf.submitting
                ]
                [ text "Cancel" ]
            , button
                [ type_ "submit"
                , class "ecc-btn ecc-btn--inline"
                , disabled (pf.submitting || not pf.dirty)
                ]
                [ text submitLabel ]
            ]
        ]


partnerFormModal : Model -> PartnerForm -> Html Msg
partnerFormModal model pf =
    let
        isEdit =
            model.editingPartnerId /= Nothing

        titleText =
            if isEdit then
                "Edit partner"

            else
                "Add partner"
    in
    div [ class "modal-backdrop", onClick RequestedClosePartnerForm ]
        [ div
            [ class "modal modal--wide"
            , Attr.attribute "role" "dialog"
            , Attr.attribute "data-draft-record-id" (Maybe.withDefault "new" model.editingPartnerId)
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
                    , onClick RequestedClosePartnerForm
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
            , if pf.confirmDiscard then
                div [ class "modal__confirm" ]
                    [ p [ class "modal__confirm-text" ]
                        [ text "Discard your changes?" ]
                    , div [ class "modal__actions" ]
                        [ button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                            , onClick CancelledClosePartnerForm
                            ]
                            [ text "Keep editing" ]
                        , button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                            , onClick ConfirmedClosePartnerForm
                            ]
                            [ text "Discard" ]
                        ]
                    ]

              else
                partnerFormView pf isEdit
            ]
        ]


deletePartnerConfirmModal : Partner -> Html Msg
deletePartnerConfirmModal partner =
    div [ class "modal-backdrop", onClick CancelledDeletePartner ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Delete partner"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( DismissedToast, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete partner" ] ]
            , p [ class "modal__confirm-text" ]
                [ text "Delete "
                , strong [] [ text partner.legalCompanyName ]
                , text "?"
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledDeletePartner
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedDeletePartner
                    ]
                    [ text "Delete" ]
                ]
            ]
        ]
