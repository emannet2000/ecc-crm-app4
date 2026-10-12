module View.Drafts exposing (view)

import Html exposing (..)
import Char
import Html.Attributes exposing (checked, class, disabled, type_)
import Html.Events exposing (onClick)
import Json.Decode as D
import Json.Encode as E
import Types exposing (..)
import View.Helpers exposing (onCheck)


values : E.Value -> List ( String, String )
values value =
    D.decodeValue (D.keyValuePairs D.value) value
        |> Result.withDefault []
        |> List.filter (\( key, _ ) -> key /= "revision")
        |> List.map (\( key, field ) -> ( key, D.decodeValue D.string field |> Result.withDefault (E.encode 0 field) ))


fieldLabel : String -> String
fieldLabel key =
    let
        words = String.toList key |> List.concatMap (\char -> if Char.isUpper char then [ ' ', Char.toLower char ] else [ char ]) |> String.fromList
    in
    case String.uncons words of
        Just ( first, rest ) -> String.fromChar (Char.toUpper first) ++ rest
        Nothing -> "Field"


view : Model -> Html Msg
view model =
    case model.draftRecovery of
        Nothing ->
            text ""

        Just recovery ->
            let
                current = values recovery.current
                changes = values recovery.fields |> List.filter (\( key, value ) -> List.member ( key, value ) current |> not)
            in
            section [ class "draft-recovery", Html.Attributes.attribute "aria-label" "Unsaved draft" ]
                [ p [ Html.Attributes.attribute "role" "status" ]
                    [ text (if recovery.restored then "Draft restored. Your entries are kept while you review changes." else "An unsaved draft is available for this record.") ]
                , div [ class "draft-recovery__actions" ]
                    [ if recovery.restored then text "" else button [ type_ "button", onClick RestoreDraft ] [ text "Restore draft" ]
                    , if recovery.restored then text "" else button [ type_ "button", onClick DiscardDraft ] [ text "Discard draft" ]
                    , if recovery.recordId == "new" then text "" else button [ type_ "button", disabled recovery.reviewing, onClick ReviewLatestDraft ] [ text (if recovery.reviewing then "Loading saved record…" else "Compare with latest saved record") ]
                    ]
                , if recovery.reviewed then
                    div []
                        [ if List.isEmpty changes then p [] [ text "Your draft matches the latest saved fields." ]
                          else div [ class "draft-comparison" ]
                            (List.map (\( key, draft ) ->
                                div [ class "draft-comparison__field" ]
                                    [ strong [] [ text (fieldLabel key) ]
                                    , p [] [ text ("Saved: " ++ (List.filter (\( k, _ ) -> k == key) current |> List.head |> Maybe.map Tuple.second |> Maybe.withDefault "—")) ]
                                    , p [] [ text ("Draft: " ++ draft) ]
                                    ]
                            ) changes)
                        , if recovery.restored then
                            label [ class "draft-recovery__review" ]
                                [ input [ type_ "checkbox", checked recovery.useLatest, onCheck UseLatestDraftRevision ] []
                                , text "I reviewed these changes and want to save using the latest record version."
                                ]
                          else text ""
                        ]
                  else text ""
                , case recovery.error of
                    Just error -> p [ Html.Attributes.attribute "role" "alert" ] [ text error ]
                    Nothing -> text ""
                ]
