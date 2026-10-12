module View.StaffPicker exposing (view, name, stat, row)

import Html exposing (Html, node, div, span, text)
import Html.Attributes as Attr
import Html.Events exposing (onInput)
import Types exposing (Msg)

view : String -> String -> Maybe String -> String -> String -> Bool -> (String -> Msg) -> Html Msg
view fieldId entity recordId selected label submitting update =
    node "crm-staff-picker"
        [ Attr.id fieldId
        , Attr.attribute "entity" entity
        , Attr.attribute "record-id" (Maybe.withDefault "" recordId)
        , Attr.attribute "value" selected
        , Attr.attribute "label" label
        , Attr.disabled submitting
        , onInput update
        ]
        []


name : String -> Html Msg
name value =
    node "crm-staff-name" [ Attr.attribute "value" value ] []


stat : String -> String -> String -> Html Msg
stat label value hint =
    div [ Attr.class "detail-stat" ]
        [ span [ Attr.class "detail-stat__label" ] [ text label ]
        , span [ Attr.class "detail-stat__value" ] [ name value ]
        , span [ Attr.class "detail-stat__hint" ] [ text hint ]
        ]


row : Html Msg -> String -> String -> Html Msg
row icon label value =
    div [ Attr.class "info-row" ]
        [ span [ Attr.class "info-row__icon" ] [ icon ]
        , div [ Attr.class "info-row__body" ]
            [ span [ Attr.class "info-row__label" ] [ text label ]
            , span [ Attr.class "info-row__value" ] [ name value ]
            ]
        ]
