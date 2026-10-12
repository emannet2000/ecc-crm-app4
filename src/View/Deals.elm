module View.Deals exposing (dealStageOptions, dealsView)

{-| Deals pipeline board.
-}

import Dict
import Html exposing (..)
import Html.Attributes as Attr exposing (class, disabled, placeholder, type_, value)
import Html.Events exposing (onClick, onInput)
import Json.Decode as D
import Json.Encode as E
import Set
import Svg
import Types exposing (..)
import View.StaffPicker as StaffPicker
import View.Contacts exposing (contactById, contactList)
import View.Filters exposing (isStale)
import View.Format exposing (dealAgeClass, dealAgeDays, dealAgeLabel, formatCurrency, formatCurrencyWith)
import View.Helpers exposing (initials, svgIcon, svgPath)
import View.Icons exposing (iconCalendar, iconEdit, iconTrash, iconUserTiny)


dealsSkeleton : Html Msg
dealsSkeleton =
    div [ class "pipeline-board" ]
        (List.repeat 4
            (div [ class "pipeline-col" ]
                [ div [ class "skeleton-line skeleton-line--short" ] []
                , div [ class "skeleton-line skeleton-line--medium" ] []
                ]
            )
        )


dealStageOptions : List String
dealStageOptions =
    [ "Lead", "Qualified", "Proposal", "Negotiation", "Won", "Lost" ]


columnDisplayCurrency : List Deal -> ( String, Bool )
columnDisplayCurrency deals =
    let
        currencies =
            deals
                |> List.map .currency
                |> List.filter (\x -> not (String.isEmpty x))
                |> List.foldl
                    (\c acc ->
                        if List.member c acc then
                            acc

                        else
                            c :: acc
                    )
                    []
    in
    case currencies of
        [] ->
            ( "USD", False )

        [ single ] ->
            ( single, False )

        _ ->
            ( "USD", True )


stageProbability : String -> Float
stageProbability stage =
    case stage of
        "Lead" ->
            0.1

        "Qualified" ->
            0.25

        "Proposal" ->
            0.5

        "Negotiation" ->
            0.75

        "Won" ->
            1.0

        "Lost" ->
            0.0

        _ ->
            0.0


weightedValue : List Deal -> Float
weightedValue deals =
    deals
        |> List.map (\d -> d.value * Maybe.withDefault (stageProbability d.stage) d.probability)
        |> List.sum


wonValue : List Deal -> Float
wonValue deals =
    deals
        |> List.filter (\d -> d.stage == "Won")
        |> List.map .value
        |> List.sum


stageNeighbors : String -> ( Maybe String, Maybe String )
stageNeighbors stage =
    let
        idx =
            dealStageOptions
                |> List.indexedMap Tuple.pair
                |> List.filter (\( _, s ) -> s == stage)
                |> List.head
                |> Maybe.map Tuple.first
                |> Maybe.withDefault 0

        prev =
            if idx == 0 then
                Nothing

            else
                List.drop (idx - 1) dealStageOptions |> List.head

        next =
            List.drop (idx + 1) dealStageOptions |> List.head
    in
    ( prev, next )


dealStageClass : String -> String
dealStageClass stage =
    case String.toLower stage of
        "lead" ->
            "pipeline-col--lead"

        "qualified" ->
            "pipeline-col--qualified"

        "proposal" ->
            "pipeline-col--proposal"

        "negotiation" ->
            "pipeline-col--negotiation"

        "won" ->
            "pipeline-col--won"

        "lost" ->
            "pipeline-col--lost"

        _ ->
            "pipeline-col--lead"


dealCard : Model -> List Contact -> Maybe String -> Deal -> Html Msg
dealCard model contacts movingId d =
    let
        isMoving =
            movingId == Just d.id

        isDragging =
            model.draggingDealId == Just d.id

        isSelected =
            Set.member d.id model.selectedDeals

        ( prevStage, nextStage ) =
            stageNeighbors d.stage

        cardClass =
            String.join " "
                [ "deal-card"
                , if isMoving then
                    "deal-card--moving"

                  else
                    ""
                , if isDragging then
                    "deal-card--dragging"

                  else
                    ""
                , if isSelected then
                    "deal-card--selected"

                  else
                    ""
                , "deal-card--selectable"
                ]

        linkedContact =
            contactById contacts d.contactId

        ageDays =
            dealAgeDays model.today d.createdAt

        ageLabel =
            dealAgeLabel ageDays

        ageClass =
            dealAgeClass ageDays

        metaItems =
            List.filterMap identity
                [ if String.isEmpty d.closeDate then
                    Nothing

                  else
                    Just
                        (span [ class "deal-card__meta-item" ]
                            [ span [ class "deal-card__meta-icon" ] [ iconCalendar ]
                            , text d.closeDate
                            ]
                        )
                , if String.isEmpty d.owner then
                    Nothing

                  else
                    Just
                        (span [ class "deal-card__meta-item" ]
                            [ span [ class "deal-card__meta-icon" ] [ iconUserTiny ]
                            , StaffPicker.name d.owner
                            ]
                        )
                ]
                |> List.intersperse
                    (span [ class "deal-card__meta-sep" ] [ text "·" ])

        moveBackMsg =
            case prevStage of
                Just s ->
                    MovedDeal d s

                Nothing ->
                    NoOp

        moveForwardMsg =
            case nextStage of
                Just s ->
                    MovedDeal d s

                Nothing ->
                    NoOp

        onCardKeyDown =
            Html.Events.preventDefaultOn "keydown"
                (D.field "key" D.string
                    |> D.andThen
                        (\key ->
                            if key == "Enter" || key == " " then
                                D.succeed ( OpenedDealDetail d, True )

                            else
                                D.fail "ignore"
                        )
                )

        onDragStart =
            Html.Events.on "dragstart"
                (D.succeed (DraggingDealStarted d.id))

        onDragEnd =
            Html.Events.on "dragend"
                (D.succeed DraggingDealEnded)
    in
    div
        [ class cardClass
        , Attr.attribute "role" "button"
        , Attr.attribute "tabindex" "0"
        , Attr.attribute "draggable" "true"
        , onClick (OpenedDealDetail d)
        , onCardKeyDown
        , onDragStart
        , onDragEnd
        ]
        [ button
            [ class
                (if isSelected then
                    "deal-card__checkbox deal-card__checkbox--checked"

                 else
                    "deal-card__checkbox"
                )
            , type_ "button"
            , Attr.attribute "role" "checkbox"
            , Attr.attribute "aria-checked"
                (if isSelected then
                    "true"

                 else
                    "false"
                )
            , Attr.attribute "aria-label" d.title
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( ToggledDealSelection d.id, True ))
            ]
            [ if isSelected then
                span [ class "deal-card__checkbox-mark" ] [ text "✓" ]

              else
                text ""
            ]
        , div [ class "deal-card__menu" ]
            [ button
                [ class "row-action"
                , type_ "button"
                , Attr.title "Edit"
                , Attr.attribute "aria-label" ("Edit " ++ d.title)
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( OpenedEditDeal d, True ))
                ]
                [ iconEdit ]
            , button
                [ class "row-action row-action--danger"
                , type_ "button"
                , Attr.title "Delete"
                , Attr.attribute "aria-label" ("Delete " ++ d.title)
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( RequestedDeleteDeal d, True ))
                ]
                [ iconTrash ]
            ]
        , div [ class "deal-card__value" ]
            [ span
                [ class
                    (if d.value == 0 then
                        "deal-card__value-amount deal-card__value-amount--zero"

                     else
                        "deal-card__value-amount"
                    )
                ]
                [ text
                    (if d.value == 0 then
                        "No value"

                     else
                        formatCurrencyWith d.currency d.value
                    )
                ]
            ]
        , div [ class "deal-card__title" ] [ text d.title ]
        , if String.isEmpty d.contactName then
            text ""

          else
            case linkedContact of
                Just c ->
                    div
                        [ class "deal-card__contact deal-card__contact--link"
                        , Attr.attribute "role" "button"
                        , Attr.attribute "tabindex" "0"
                        , Attr.title ("Open " ++ c.name)
                        , Html.Events.stopPropagationOn "click"
                            (D.succeed ( OpenedContactDetail c, True ))
                        , Html.Events.preventDefaultOn "keydown"
                            (D.field "key" D.string
                                |> D.andThen
                                    (\key ->
                                        if key == "Enter" || key == " " then
                                            D.succeed ( OpenedContactDetail c, True )

                                        else
                                            D.fail "ignore"
                                    )
                            )
                        ]
                        [ div [ class "deal-card__contact-avatar" ]
                            [ text (initials c.name) ]
                        , span [ class "deal-card__contact-name" ]
                            [ text c.name ]
                        ]

                Nothing ->
                    div [ class "deal-card__contact" ]
                        [ div [ class "deal-card__contact-avatar" ]
                            [ text (initials d.contactName) ]
                        , span [ class "deal-card__contact-name" ]
                            [ text d.contactName ]
                        ]
        , div [ class "deal-card__footer" ]
            [ div [ class "deal-card__meta" ]
                metaItems
            , div [ class "deal-card__move" ]
                [ button
                    [ class "deal-move-btn"
                    , type_ "button"
                    , Attr.title "Move back a stage"
                    , Attr.attribute "aria-label" ("Move " ++ d.title ++ " back")
                    , Html.Events.stopPropagationOn "click"
                        (D.succeed ( moveBackMsg, True ))
                    , disabled (isMoving || prevStage == Nothing)
                    ]
                    [ text "←" ]
                , button
                    [ class "deal-move-btn"
                    , type_ "button"
                    , Attr.title "Move forward a stage"
                    , Attr.attribute "aria-label" ("Move " ++ d.title ++ " forward")
                    , Html.Events.stopPropagationOn "click"
                        (D.succeed ( moveForwardMsg, True ))
                    , disabled (isMoving || nextStage == Nothing)
                    ]
                    [ text "→" ]
                ]
            ]
        ]


dealColumn : Model -> List Contact -> Maybe String -> String -> List Deal -> Html Msg
dealColumn model contacts movingId stageName deals =
    let
        total =
            deals |> List.map .value |> List.sum

        weighted =
            weightedValue deals

        isEmpty =
            case model.deals of
                Success data ->
                    (Dict.get stageName data.summary.stages |> Maybe.map .count |> Maybe.withDefault 0) == 0

                _ ->
                    List.isEmpty deals

        column =
            case model.deals of
                Success data ->
                    Dict.get stageName data.summary.stages |> Maybe.withDefault { count = 0, values = Dict.empty }

                _ ->
                    { count = List.length deals, values = Dict.empty }

        count =
            column.count

        ( displayCurrency, isMixed ) =
            columnDisplayCurrency deals

        totalTitle =
            if isMixed then
                "Separate totals by currency; no exchange rate is assumed"

            else
                ""

        isDropTarget =
            model.dropTargetStage == Just stageName

        colClass =
            String.join " "
                [ "pipeline-col"
                , dealStageClass stageName
                , if isDropTarget then
                    "pipeline-col--drop-target"

                  else
                    ""
                ]

        totalClass =
            if isEmpty then
                "pipeline-col__total pipeline-col__total--muted"

            else
                "pipeline-col__total"

        maxTotal =
            100000

        barWidth =
            if total == 0 then
                "0%"

            else
                String.fromFloat
                    (min 100 (total / maxTotal * 100))
                    ++ "%"

        onDragOver =
            Html.Events.preventDefaultOn "dragover"
                (D.succeed ( DropTargetEntered stageName, True ))

        onDragLeave =
            Html.Events.on "dragleave"
                (D.succeed DropTargetLeft)

        onDropEvent =
            Html.Events.preventDefaultOn "drop"
                (D.succeed ( DealDroppedOnStage stageName, True ))
    in
    div
        [ class colClass
        , onDragOver
        , onDragLeave
        , onDropEvent
        ]
        [ div [ class "pipeline-col__header" ]
            [ div [ class "pipeline-col__title-wrap" ]
                [ span [ class "pipeline-col__dot" ] []
                , span [ class "pipeline-col__title" ] [ text stageName ]
                , span [ class "pipeline-col__count" ]
                    [ text (String.fromInt count) ]
                ]
            , button
                [ class "pipeline-col__add"
                , type_ "button"
                , Attr.title ("Add deal to " ++ stageName)
                , Attr.attribute "aria-label" ("Add deal to " ++ stageName)
                , onClick (OpenedAddDealWithStage stageName)
                ]
                [ svgIcon
                    [ Attr.attribute "viewBox" "0 0 24 24"
                    , Attr.attribute "width" "14"
                    , Attr.attribute "height" "14"
                    , Attr.attribute "fill" "none"
                    , Attr.attribute "stroke" "currentColor"
                    , Attr.attribute "stroke-width" "2.2"
                    , Attr.attribute "stroke-linecap" "round"
                    , Attr.attribute "stroke-linejoin" "round"
                    ]
                    [ svgPath "M12 5v14"
                    , svgPath "M5 12h14"
                    ]
                ]
            ]
        , div [ class "pipeline-col__totals" ]
            [ span
                [ class totalClass
                , Attr.title totalTitle
                ]
                [ text
                    (if isEmpty then
                        "—"

                     else
                        column.values |> Dict.toList |> List.map (\( currency, amount ) -> formatCurrencyWith currency amount) |> String.join " · "
                    )
                ]
            ]
        , if isEmpty then
            div [ class "pipeline-col__empty" ]
                [ div [ class "pipeline-col__empty-icon" ]
                    [ svgIcon
                        [ Attr.attribute "viewBox" "0 0 24 24"
                        , Attr.attribute "width" "14"
                        , Attr.attribute "height" "14"
                        , Attr.attribute "fill" "none"
                        , Attr.attribute "stroke" "currentColor"
                        , Attr.attribute "stroke-width" "1.8"
                        , Attr.attribute "stroke-linecap" "round"
                        , Attr.attribute "stroke-linejoin" "round"
                        ]
                        [ svgPath "M12 5v14"
                        , svgPath "M5 12h14"
                        ]
                    ]
                , text "No deals here"
                ]

          else
            div [ class "pipeline-col__cards" ]
                (List.map (dealCard model contacts movingId) deals)
        ]


pipelineBoard : Model -> List Contact -> Maybe String -> List Deal -> Html Msg
pipelineBoard model contacts movingId deals =
    div [ class "pipeline-board" ]
        (dealStageOptions
            |> List.map
                (\stageName ->
                    dealColumn model
                        contacts
                        movingId
                        stageName
                        (List.filter (\d -> d.stage == stageName) deals)
                )
        )


kpiCard : String -> String -> String -> List (Html Msg) -> Html Msg
kpiCard label valueText modifierClass hintChildren =
    div [ class ("deals-summary__kpi " ++ modifierClass) ]
        [ span [ class "deals-summary__label" ] [ text label ]
        , span [ class "deals-summary__value" ] [ text valueText ]
        , div [ class "deals-summary__hint" ] hintChildren
        ]


dealsView : Model -> Html Msg
dealsView model =
    case model.deals of
        NotAsked ->
            dealsSkeleton

        Loading ->
            dealsSkeleton

        Failure msg ->
            div [ class "content__empty-block" ] [ text ("Could not load deals: " ++ msg), button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick (NavigatedTo model.route) ] [ text "Retry" ] ]

        Success data ->
            let
                visibleDeals =
                    data.items

                count =
                    data.total

                openDeals =
                    List.filter (\d -> d.stage /= "Won" && d.stage /= "Lost") visibleDeals

                totalValue =
                    visibleDeals |> List.map .value |> List.sum

                weighted =
                    weightedValue visibleDeals

                wonTotal =
                    wonValue visibleDeals

                wonCount =
                    data.summary.wonCount

                avgValue =
                    if count == 0 then
                        0

                    else
                        totalValue / toFloat count

                conversionRate =
                    if count == 0 then
                        0

                    else
                        (toFloat wonCount / toFloat count) * 100

                isQueryEmpty =
                    String.isEmpty (String.trim data.query)

                isSearching =
                    model.pendingDealsQuery /= Nothing

                summaryClass =
                    if isSearching then
                        "deals-summary deals-summary--loading"

                    else
                        "deals-summary"

                selectionCount =
                    Set.size model.selectedDeals

                owners =
                    data.owners

                hasActiveFilters =
                    not (String.isEmpty model.ownerFilter)
                        || not (String.isEmpty model.dateFromFilter)
                        || not (String.isEmpty model.dateToFilter)
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
                            , Attr.attribute "aria-label" "Search deals"
                            , placeholder "Search deals…"
                            , value data.query
                            , onInput UpdatedDealsQuery
                            ]
                            []
                        , if isSearching then
                            span [ class "page-toolbar__spinner" ] []

                          else
                            text ""
                        ]
                    , button
                        [ class "ecc-btn ecc-btn--inline"
                        , type_ "button"
                        , onClick OpenedAddDeal
                        ]
                        [ text "Add deal" ]
                    ]
                , details
                    ([ class "deals-filter-panel" ] ++ (if hasActiveFilters then [ Attr.attribute "open" "" ] else []))
                    [ summary [ class "deals-filter-panel__toggle" ]
                        [ text (if hasActiveFilters then "Filters · active" else "Filters · staff and closing date") ]
                    , div [ class "deals-filters" ]
                    [ div [ class "deals-filters__group" ]
                        [ button [ class "ecc-btn ecc-btn--ghost", onClick (UpdatedOwnerFilter "__mine__") ] [ text "My deals" ]
                        , span [ class "deals-filters__label" ] [ text "Owner" ]
                        , node "crm-staff-filter"
                            [ onInput UpdatedOwnerFilter
                            , Attr.attribute "value" model.ownerFilter
                            , Attr.attribute "owners" (E.encode 0 (E.list E.string owners))
                            ]
                            []
                        ]
                    , div [ class "deals-filters__group" ]
                        [ span [ class "deals-filters__label" ] [ text "Close from" ]
                        , input
                            [ type_ "date"
                            , value model.dateFromFilter
                            , onInput UpdatedDateFromFilter
                            ]
                            []
                        ]
                    , div [ class "deals-filters__group" ]
                        [ span [ class "deals-filters__label" ] [ text "to" ]
                        , input
                            [ type_ "date"
                            , value model.dateToFilter
                            , onInput UpdatedDateToFilter
                            ]
                            []
                        ]
                    , span [ class "deals-filters__count" ]
                        [ text
                            (if hasActiveFilters then
                                String.fromInt count
                                    ++ " of "
                                    ++ String.fromInt data.total
                                    ++ " shown"

                             else
                                String.fromInt data.total
                                    ++ " deal"
                                    ++ (if data.total == 1 then
                                            ""

                                        else
                                            "s"
                                       )
                            )
                        ]
                    , if hasActiveFilters then
                        button
                            [ class "deals-filters__clear"
                            , type_ "button"
                            , onClick ClearedDealFilters
                            ]
                            [ text "Clear filters" ]

                      else
                        text ""
                    ]
                    ]
                , if selectionCount > 0 then
                    div [ class "deals-bulk-bar" ]
                        [ span [ class "deals-bulk-bar__count" ]
                            [ text
                                (String.fromInt selectionCount
                                    ++ " selected"
                                )
                            ]
                        , span [ class "deals-bulk-bar__spacer" ] []
                        , div [ class "deals-bulk-bar__actions" ]
                            [ button
                                [ class "deals-bulk-bar__btn"
                                , type_ "button"
                                , onClick (RequestedBulkMove "")
                                ]
                                [ text "Move to stage…" ]
                            , button
                                [ class "deals-bulk-bar__btn deals-bulk-bar__btn--danger"
                                , type_ "button"
                                , onClick RequestedBulkDelete
                                ]
                                [ text "Delete" ]
                            ]
                        , button
                            [ class "deals-bulk-bar__close"
                            , type_ "button"
                            , Attr.attribute "aria-label" "Clear selection"
                            , onClick ClearedDealSelection
                            ]
                            [ text "×" ]
                        ]

                  else
                    text ""
                , details [ class "deals-extra-summary" ]
                    [ summary [] [ text "Pipeline metrics" ]
                    , div [ class summaryClass ]
                    [ kpiCard "Open pipeline"
                        (boardAmounts .open data.summary.currencies)
                        "deals-summary__kpi--total"
                        [ text
                            (String.fromInt data.summary.openCount
                                ++ " open deal"
                                ++ (if data.summary.openCount == 1 then
                                        ""

                                    else
                                        "s"
                                   )
                            )
                        ]
                    , kpiCard "Expected value"
                        (boardAmounts .weighted data.summary.currencies)
                        "deals-summary__kpi--weighted"
                        [ text "Stage-adjusted" ]
                    , kpiCard "Won value"
                        (boardAmounts .won data.summary.currencies)
                        "deals-summary__kpi--won"
                        [ text (String.fromInt wonCount ++ " closed") ]
                    ]
                    , div [ class "deals-summary" ]
                    [ kpiCard "Average deal value"
                        (boardAmounts (\currency -> if currency.count == 0 then 0 else currency.total / toFloat currency.count) data.summary.currencies)
                        "deals-summary__kpi--avg"
                        [ text "Per deal" ]
                    , kpiCard "Conversion"
                        (String.fromInt (round conversionRate) ++ "%")
                        "deals-summary__kpi--convert"
                        [ text "Won / total" ]
                    ]

                    ]
                , if List.isEmpty visibleDeals then
                    div [ class "empty-state" ]
                        [ h3 [ class "empty-state__title" ]
                            [ text
                                (if isQueryEmpty && not hasActiveFilters then
                                    "No deals yet"

                                 else
                                    "No deals match your filters"
                                )
                            ]
                        , p [ class "empty-state__desc" ]
                            [ text
                                (if isQueryEmpty && not hasActiveFilters then
                                    "Add your first deal to start tracking your pipeline."

                                 else
                                    "Try adjusting your search or filters."
                                )
                            ]
                        , if isQueryEmpty && not hasActiveFilters then
                            div [ class "empty-state__action" ]
                                [ button
                                    [ class "ecc-btn ecc-btn--inline"
                                    , type_ "button"
                                    , onClick OpenedAddDeal
                                    ]
                                    [ text "Add your first deal" ]
                                ]

                          else
                            text ""
                        ]

                  else
                    div [ class "deals-pipeline" ]
                        [ div [ class "deals-pipeline__heading" ]
                            [ h3 [] [ text "Deals by stage" ]
                            , p [] [ text "Open a deal for details. Drag cards or use the arrows to change stage." ]
                            ]
                        , p [ class "deals-page-scope" ]
                            [ text (String.fromInt (List.length visibleDeals) ++ " of " ++ String.fromInt count ++ " matching deals loaded · metrics cover all matches") ]
                        , pipelineBoard model (contactList model) model.movingDealId visibleDeals
                        , div [ class "deals-load-actions" ]
                            [ if List.length data.items < data.total then
                                button [ class "ecc-btn ecc-btn--inline", disabled data.loadingMore, onClick LoadMoreDeals ] [ text (if data.loadingMore then "Loading more…" else "Load more deals") ]
                              else
                                text "All matching cards loaded"
                            , button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick RefreshDeals ] [ text "Refresh board" ]
                            ]
                        ]
                ]


currencyTotals : (Deal -> Float) -> List Deal -> String
currencyTotals amount deals =
    deals
        |> List.foldl
            (\deal totals ->
                Dict.update
                    (if String.isEmpty deal.currency then
                        "USD"

                     else
                        deal.currency
                    )
                    (\current -> Just (Maybe.withDefault 0 current + amount deal))
                    totals
            )
            Dict.empty
        |> Dict.toList
        |> List.map (\( currency, total ) -> formatCurrencyWith currency total)
        |> String.join " · "


boardAmounts : (BoardCurrency -> Float) -> Dict.Dict String BoardCurrency -> String
boardAmounts amount currencies =
    if Dict.isEmpty currencies then
        "—"

    else
        currencies |> Dict.toList |> List.map (\( currency, totals ) -> formatCurrencyWith currency (amount totals)) |> String.join " · "
