module View.Icons exposing (eccMark, iconAgent, iconBack, iconCalendar, iconChat, iconContacts, iconDeals, iconEdit, iconHome, iconLead, iconMail, iconMoon, iconPhone, iconPin, iconRefresh, iconReports, iconSchool, iconSearch, iconSettings, iconSignOut, iconStudent, iconSun, iconTasks, iconTrash, iconUserTiny)

{-| Brand mark and all SVG icons.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class)
import Svg
import Types exposing (..)
import View.Helpers exposing (svgIcon, svgPath)


eccMark : Html msg
eccMark =
    div [ class "ecc-mark" ]
        [ svgIcon
            [ Attr.attribute "viewBox" "0 0 40 40"
            , Attr.attribute "width" "40"
            , Attr.attribute "height" "40"
            , Attr.attribute "fill" "none"
            ]
            [ Svg.node "rect"
                [ Attr.attribute "x" "0"
                , Attr.attribute "y" "0"
                , Attr.attribute "width" "40"
                , Attr.attribute "height" "40"
                , Attr.attribute "rx" "12"
                , Attr.attribute "fill" "url(#eccGrad)"
                ]
                []
            , Svg.node "text"
                [ Attr.attribute "x" "20"
                , Attr.attribute "y" "26"
                , Attr.attribute "text-anchor" "middle"
                , Attr.attribute "font-family" "Inter, sans-serif"
                , Attr.attribute "font-size" "16"
                , Attr.attribute "font-weight" "700"
                , Attr.attribute "fill" "white"
                , Attr.attribute "letter-spacing" "0.5"
                ]
                [ text "ECC" ]
            , Svg.node "defs"
                []
                [ Svg.node "linearGradient"
                    [ Attr.attribute "id" "eccGrad"
                    , Attr.attribute "x1" "0"
                    , Attr.attribute "y1" "0"
                    , Attr.attribute "x2" "40"
                    , Attr.attribute "y2" "40"
                    , Attr.attribute "gradientUnits" "userSpaceOnUse"
                    ]
                    [ Svg.node "stop"
                        [ Attr.attribute "offset" "0"
                        , Attr.attribute "stop-color" "#6366F1"
                        ]
                        []
                    , Svg.node "stop"
                        [ Attr.attribute "offset" "1"
                        , Attr.attribute "stop-color" "#06B6D4"
                        ]
                        []
                    ]
                ]
            ]
        ]


iconHome : Html Msg
iconHome =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "18"
        , Attr.attribute "height" "18"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"
        , svgPath "M9 22V12h6v10"
        ]


iconContacts : Html Msg
iconContacts =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "18"
        , Attr.attribute "height" "18"
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


iconDeals : Html Msg
iconDeals =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "18"
        , Attr.attribute "height" "18"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"
        ]


iconTasks : Html Msg
iconTasks =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "18"
        , Attr.attribute "height" "18"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M9 11l3 3L22 4"
        , svgPath "M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"
        ]


iconReports : Html Msg
iconReports =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "18"
        , Attr.attribute "height" "18"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M18 20V10M12 20V4M6 20v-6"
        ]


iconSettings : Html Msg
iconSettings =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "18"
        , Attr.attribute "height" "18"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ Svg.node "circle"
            [ Attr.attribute "cx" "12"
            , Attr.attribute "cy" "12"
            , Attr.attribute "r" "3"
            ]
            []
        , svgPath "M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"
        ]


iconSchool : Html Msg
iconSchool =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "18"
        , Attr.attribute "height" "18"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M22 10v6M2 10l10-5 10 5-10 5z"
        , svgPath "M6 12v5c3 3 9 3 12 0v-5"
        ]


iconStudent : Html Msg
iconStudent =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "18"
        , Attr.attribute "height" "18"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M12 14l9-5-9-5-9 5 9 5z"
        , svgPath "M12 14l6.16-3.42a12 12 0 0 1-12.32 0z"
        , svgPath "M12 14v7"
        ]


iconAgent : Html Msg
iconAgent =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "18"
        , Attr.attribute "height" "18"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"
        , Svg.node "circle"
            [ Attr.attribute "cx" "8.5"
            , Attr.attribute "cy" "7"
            , Attr.attribute "r" "4"
            ]
            []
        , svgPath "M20 8v6M23 11h-6"
        ]


iconLead : Html Msg
iconLead =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "18"
        , Attr.attribute "height" "18"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M22 12h-4l-3 9L9 3l-3 9H2"
        ]


iconSignOut : Html Msg
iconSignOut =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "16"
        , Attr.attribute "height" "16"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"
        , svgPath "M16 17l5-5-5-5"
        , svgPath "M21 12H9"
        ]


iconEdit : Html msg
iconEdit =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "15"
        , Attr.attribute "height" "15"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"
        , svgPath "M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"
        ]


iconTrash : Html msg
iconTrash =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "15"
        , Attr.attribute "height" "15"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M3 6h18"
        , svgPath "M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"
        , svgPath "M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
        ]


iconBack : Html msg
iconBack =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "16"
        , Attr.attribute "height" "16"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "2"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M19 12H5"
        , svgPath "M12 19l-7-7 7-7"
        ]


iconMail : Html msg
iconMail =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "14"
        , Attr.attribute "height" "14"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"
        , svgPath "M22 6l-10 7L2 6"
        ]


iconPhone : Html msg
iconPhone =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "14"
        , Attr.attribute "height" "14"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z"
        ]


iconCalendar : Html msg
iconCalendar =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "14"
        , Attr.attribute "height" "14"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ Svg.node "rect"
            [ Attr.attribute "x" "3"
            , Attr.attribute "y" "4"
            , Attr.attribute "width" "18"
            , Attr.attribute "height" "18"
            , Attr.attribute "rx" "2"
            ]
            []
        , svgPath "M16 2v4M8 2v4M3 10h18"
        ]


iconPin : Html msg
iconPin =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "14"
        , Attr.attribute "height" "14"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0 1 18 0z"
        , Svg.node "circle"
            [ Attr.attribute "cx" "12"
            , Attr.attribute "cy" "10"
            , Attr.attribute "r" "3"
            ]
            []
        ]


iconUserTiny : Html msg
iconUserTiny =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "11"
        , Attr.attribute "height" "11"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "2"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"
        , Svg.node "circle"
            [ Attr.attribute "cx" "12"
            , Attr.attribute "cy" "7"
            , Attr.attribute "r" "4"
            ]
            []
        ]


iconLock : Html msg
iconLock =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "14"
        , Attr.attribute "height" "14"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ Svg.node "rect"
            [ Attr.attribute "x" "3"
            , Attr.attribute "y" "11"
            , Attr.attribute "width" "18"
            , Attr.attribute "height" "11"
            , Attr.attribute "rx" "2"
            ]
            []
        , svgPath "M7 11V7a5 5 0 0 1 10 0v4"
        ]


outlineIcon : List String -> Html Msg
outlineIcon paths =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "18"
        , Attr.attribute "height" "18"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        , Attr.attribute "aria-hidden" "true"
        ]
        (List.map svgPath paths)


iconMoon : Html Msg
iconMoon =
    outlineIcon [ "M21 12.8A9 9 0 0 1 11.2 3 9 9 0 1 0 21 12.8Z" ]


iconSun : Html Msg
iconSun =
    outlineIcon [ "M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0", "M12 2v2m0 16v2M2 12h2m16 0h2M4.93 4.93l1.42 1.42m11.3 11.3 1.42 1.42M4.93 19.07l1.42-1.42m11.3-11.3 1.42-1.42" ]


iconSearch : Html Msg
iconSearch =
    outlineIcon [ "M19 11a8 8 0 1 1-16 0 8 8 0 0 1 16 0", "m21 21-4.4-4.4" ]


iconRefresh : Html Msg
iconRefresh =
    outlineIcon [ "M20 7v5h-5", "M4 17v-5h5", "M6.1 6.1A8 8 0 0 1 19.3 9L20 12M4 12l.7 3A8 8 0 0 0 17.9 17.9" ]


iconChat : Html Msg
iconChat =
    svgIcon
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "18"
        , Attr.attribute "height" "18"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "1.8"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ svgPath "M21 11.5a8.4 8.4 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.4 8.4 0 0 1-3.8-.9L3 21l1.9-5.7a8.4 8.4 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6A8.4 8.4 0 0 1 12.5 3H13a8.5 8.5 0 0 1 8 8v.5Z" ]
