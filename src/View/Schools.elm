module View.Schools exposing (deleteSchoolConfirmModal, schoolDetailView, schoolFormModal, schoolsView)

{-| Schools: list, detail, form.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class, disabled, for, id, placeholder, type_, value)
import Html.Events exposing (onClick, onInput, onSubmit)
import Json.Decode as D
import Router exposing (routeToPath)
import Svg
import Types exposing (..)
import View.Helpers exposing (detailCard, detailEmpty, detailStat, infoRow, initials, paginationBar, svgIcon, svgPath)
import View.Icons exposing (iconBack, iconCalendar, iconContacts, iconDeals, iconEdit, iconPin, iconTrash, iconUserTiny)
import View.Students exposing (linkedStudentsView)


contractStatusBadge : String -> Html Msg
contractStatusBadge status =
    let
        cls =
            case String.toLower status of
                "signed" ->
                    "badge badge--success"

                "pending" ->
                    "badge badge--info"

                "follow-up required" ->
                    "badge badge--muted"

                _ ->
                    "badge"
    in
    span [ class cls ] [ text status ]


schoolRow : School -> Html Msg
schoolRow s =
    tr [ class "contact-row" ]
        [ td []
            [ div [ class "contact-name-cell" ]
                [ div [ class "contact-avatar" ] [ text (initials s.name) ]
                , div [ class "contact-name-info" ]
                    [ a [ class "contact-name record-link", Attr.href (routeToPath (SchoolDetail s.id)) ] [ text s.name ]
                    , span [ class "contact-email" ]
                        [ text
                            (if String.isEmpty s.website then
                                s.countryCode

                             else
                                s.website
                            )
                        ]
                    ]
                ]
            ]
        , td [] [ text s.countryCode ]
        , td [] [ text s.commissionRate ]
        , td [] [ contractStatusBadge s.contractStatus ]
        , td [] [ text (String.fromInt s.studentsEnrolled) ]
        , td [ class "contact-actions-cell" ]
            [ button
                [ class "row-action"
                , type_ "button"
                , Attr.title "Edit"
                , Attr.attribute "aria-label" ("Edit " ++ s.name)
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( OpenedEditSchool s, True ))
                ]
                [ iconEdit ]
            , button
                [ class "row-action row-action--danger"
                , type_ "button"
                , Attr.title "Delete"
                , Attr.attribute "aria-label" ("Delete " ++ s.name)
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( RequestedDeleteSchool s, True ))
                ]
                [ iconTrash ]
            ]
        ]


schoolsSkeleton : Html Msg
schoolsSkeleton =
    div []
        [ div [ class "page-toolbar" ]
            [ div [ class "page-toolbar__search" ]
                [ input [ type_ "text", placeholder "Search schools…", disabled True ] [] ]
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


schoolsView : Model -> Html Msg
schoolsView model =
    case model.schools of
        NotAsked ->
            schoolsSkeleton

        Loading ->
            schoolsSkeleton

        Failure msg ->
            div [ class "content__empty-block" ]
                [ text ("Could not load schools: " ++ msg), button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick (NavigatedTo model.route) ] [ text "Retry" ] ]

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
                            , Attr.attribute "aria-label" "Search schools"
                            , placeholder "Search schools…"
                            , value data.query
                            , onInput UpdatedSchoolsQuery
                            ]
                            []
                        ]
                    , button
                        [ class "ecc-btn ecc-btn--inline"
                        , type_ "button"
                        , onClick OpenedAddSchool
                        ]
                        [ text "Add school" ]
                    ]
                , if List.isEmpty filtered then
                    div [ class "empty-state" ]
                        [ h3 [ class "empty-state__title" ]
                            [ text
                                (if isQueryEmpty then
                                    "No schools yet"

                                 else
                                    "No schools match your search"
                                )
                            ]
                        , p [ class "empty-state__desc" ]
                            [ text
                                (if isQueryEmpty then
                                    "Add your first partner school to start tracking referrals."

                                 else
                                    "Try a different search term."
                                )
                            ]
                        , if isQueryEmpty then
                            div [ class "empty-state__action" ]
                                [ button
                                    [ class "ecc-btn ecc-btn--inline"
                                    , type_ "button"
                                    , onClick OpenedAddSchool
                                    ]
                                    [ text "Add your first school" ]
                                ]

                          else
                            text ""
                        ]

                  else
                    div [ class "table-wrap" ]
                        [ table [ class "data-table" ]
                            [ thead []
                                [ tr []
                                    [ th [] [ text "School" ]
                                    , th [] [ text "Country" ]
                                    , th [] [ text "Commission" ]
                                    , th [] [ text "Contract" ]
                                    , th [] [ text "Linked students" ]
                                    , th [ class "th-actions" ] [ text "" ]
                                    ]
                                ]
                            , tbody [] (List.map schoolRow filtered)
                            ]
                        ]
                , paginationBar data.total data.offset data.limit SchoolsPageChanged
                ]


schoolDetailView : Model -> School -> Html Msg
schoolDetailView model s =
    let
        display v =
            if String.isEmpty v then
                "—"

            else
                v

        websiteDisplay =
            display s.website

        contactDisplay =
            display s.contactPerson

        commissionDisplay =
            display s.commissionRate

        createdDisplay =
            display s.createdAt

        ownerDisplay =
            display s.createdBy

        linkedCount =
            case model.linkedStudents of
                Success data ->
                    if data.parentId == s.id then
                        data.total

                    else
                        s.studentsEnrolled

                _ ->
                    s.studentsEnrolled
    in
    div [ class "detail" ]
        [ button
            [ class "detail__back"
            , type_ "button"
            , onClick (NavigatedTo Schools)
            ]
            [ iconBack
            , span [] [ text "Back to schools" ]
            ]
        , header [ class "detail-hero" ]
            [ div [ class "detail-hero__avatar" ] [ text (initials s.name) ]
            , div [ class "detail-hero__body" ]
                [ div [ class "detail-hero__title-row" ]
                    [ h1 [ class "detail-hero__name" ] [ text s.name ]
                    , contractStatusBadge s.contractStatus
                    ]
                , p [ class "detail-hero__role" ]
                    [ text (display s.countryCode) ]
                ]
            , div [ class "detail-hero__actions" ]
                [ button
                    [ class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , type_ "button"
                    , onClick (OpenedEditSchool s)
                    ]
                    [ iconEdit
                    , span [] [ text "Edit" ]
                    ]
                , button
                    [ class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , type_ "button"
                    , onClick (RequestedDeleteSchool s)
                    ]
                    [ iconTrash
                    , span [] [ text "Delete" ]
                    ]
                ]
            ]
        , div [ class "detail-stats" ]
            [ detailStat "Linked students"
                (String.fromInt linkedCount)
                "Applications associated"
            , detailStat "Commission" commissionDisplay "Rate"
            , detailStat "Country" (display s.countryCode) "Location"
            , detailStat "Created" createdDisplay "Added to CRM"
            ]
        , div [ class "detail__grid" ]
            [ aside [ class "detail__sidebar" ]
                [ detailCard "About"
                    Nothing
                    (div [ class "info-list" ]
                        [ infoRow iconPin "Country" (display s.countryCode)
                        , infoRow iconUserTiny "Contact" contactDisplay
                        , infoRow iconDeals "Commission" commissionDisplay
                        , infoRow iconCalendar "Created" createdDisplay
                        , infoRow iconContacts "Added by" ownerDisplay
                        ]
                    )
                , detailCard "Website"
                    Nothing
                    (if String.isEmpty s.website then
                        p [ class "detail-muted" ] [ text "No website on file." ]

                     else
                        p [ class "detail-notes" ]
                            [ a [ Attr.href s.website ] [ text s.website ] ]
                    )
                , detailCard "Notes"
                    Nothing
                    (if String.isEmpty s.notes then
                        p [ class "detail-muted" ] [ text "No notes yet." ]

                     else
                        p [ class "detail-notes" ] [ text s.notes ]
                    )
                ]
            , div [ class "detail__main" ]
                [ detailCard "Students"
                    (Just
                        (button
                            [ class "detail-card__action"
                            , type_ "button"
                            , onClick (OpenedAddStudentForSchool s)
                            ]
                            [ text "Add student" ]
                        )
                    )
                    (div []
                        [ linkedStudentsView model s.id
                        , button
                            [ class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                            , type_ "button"
                            , onClick (NavigatedTo Students)
                            ]
                            [ text "Open students" ]
                        ]
                    )
                ]
            ]
        ]


schoolFormFieldError : String -> SchoolForm -> Maybe String
schoolFormFieldError field sf =
    sf.errors
        |> List.filter (\( f, _ ) -> f == field)
        |> List.head
        |> Maybe.map Tuple.second


schoolRichField : SchoolForm -> String -> String -> String -> Html Msg
schoolRichField sf fieldId labelText inputType =
    let
        currentValue =
            case fieldId of
                "name" ->
                    sf.name

                "countryCode" ->
                    sf.countryCode

                "commissionRate" ->
                    sf.commissionRate

                "studentsEnrolled" ->
                    sf.studentsEnrolled

                "contactPerson" ->
                    sf.contactPerson

                "website" ->
                    sf.website

                _ ->
                    ""

        err =
            schoolFormFieldError fieldId sf

        cls =
            case err of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"
    in
    div [ class cls ]
        ([ input
            [ id ("sf-" ++ fieldId)
            , type_ inputType
            , placeholder " "
            , value currentValue
            , onInput (UpdatedSchoolFormField fieldId)
            , disabled sf.submitting
            ]
            []
         , label [ for ("sf-" ++ fieldId) ] [ text labelText ]
         ]
            ++ (case err of
                    Just msg ->
                        [ p [ class "ecc-field__message" ] [ text msg ] ]

                    Nothing ->
                        []
               )
        )


schoolContractPills : SchoolForm -> Html Msg
schoolContractPills sf =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Contract status" ]
        , div [ class "stage-pills" ]
            (List.map
                (\s ->
                    let
                        cls =
                            if sf.contractStatus == s then
                                "stage-pill stage-pill--active"

                            else
                                "stage-pill"
                    in
                    button
                        [ type_ "button"
                        , class cls
                        , onClick (UpdatedSchoolFormField "contractStatus" s)
                        , disabled sf.submitting
                        ]
                        [ text s ]
                )
                [ "Signed", "Pending", "Follow-up Required" ]
            )
        , case schoolFormFieldError "contractStatus" sf of
            Just msg ->
                p [ class "ecc-field__message" ] [ text msg ]

            Nothing ->
                text ""
        ]


schoolFormView : SchoolForm -> Bool -> Html Msg
schoolFormView sf isEdit =
    let
        formError =
            schoolFormFieldError "form" sf

        submitLabel =
            if sf.submitting then
                "Saving…"

            else if isEdit then
                "Save changes"

            else
                "Save school"
    in
    form [ onSubmit SubmittedSchoolForm, Attr.novalidate True ]
        [ Html.node "crm-draft-recovery" [] []
        , case formError of
            Just msg ->
                div [ class "ecc-alert ecc-alert--error" ] [ text msg ]

            Nothing ->
                text ""
        , div [ class "form-grid" ]
            [ schoolRichField sf "name" "School name" "text"
            , schoolRichField sf "countryCode" "Country code (e.g. CA)" "text"
            , schoolRichField sf "commissionRate" "Commission (e.g. 15%)" "text"
            , schoolRichField sf "contactPerson" "Contact person" "text"
            , schoolRichField sf "website" "Website" "url"
            ]
        , schoolContractPills sf
        , div [ class "ecc-field ecc-field--notes" ]
            [ span [ class "ecc-field__label" ] [ text "Notes" ]
            , textarea
                [ id "sf-notes"
                , placeholder "Additional notes or remarks…"
                , value sf.notes
                , onInput (UpdatedSchoolFormField "notes")
                , disabled sf.submitting
                , Attr.rows 4
                ]
                []
            ]
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick RequestedCloseSchoolForm
                , disabled sf.submitting
                ]
                [ text "Cancel" ]
            , button
                [ type_ "submit"
                , class "ecc-btn ecc-btn--inline"
                , disabled (sf.submitting || not sf.dirty)
                ]
                [ text submitLabel ]
            ]
        ]


schoolFormModal : Model -> SchoolForm -> Html Msg
schoolFormModal model sf =
    let
        isEdit =
            model.editingSchoolId /= Nothing

        titleText =
            if isEdit then
                "Edit school"

            else
                "Add school"
    in
    div [ class "modal-backdrop", onClick RequestedCloseSchoolForm ]
        [ div
            [ class "modal modal--wide"
            , Attr.attribute "role" "dialog"
            , Attr.attribute "data-draft-record-id" (Maybe.withDefault "new" model.editingSchoolId)
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
                    , onClick RequestedCloseSchoolForm
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
            , if sf.confirmDiscard then
                div [ class "modal__confirm" ]
                    [ p [ class "modal__confirm-text" ]
                        [ text "Discard your changes? They won't be saved." ]
                    , div [ class "modal__actions" ]
                        [ button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                            , onClick CancelledCloseSchoolForm
                            ]
                            [ text "Keep editing" ]
                        , button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                            , onClick ConfirmedCloseSchoolForm
                            ]
                            [ text "Discard" ]
                        ]
                    ]

              else
                schoolFormView sf isEdit
            ]
        ]


deleteSchoolConfirmModal : School -> Html Msg
deleteSchoolConfirmModal school =
    div [ class "modal-backdrop", onClick CancelledDeleteSchool ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Delete school"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( DismissedToast, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete school" ]
                , button
                    [ class "modal__close"
                    , type_ "button"
                    , onClick CancelledDeleteSchool
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
                , strong [] [ text school.name ]
                , text "? This cannot be undone."
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledDeleteSchool
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedDeleteSchool
                    ]
                    [ text "Delete" ]
                ]
            ]
        ]
