module Update.Activities exposing (update)

{-| Activity messages.
-}

import Api
import Types exposing (..)
import Update.Loaders exposing (loadActivities, loadContacts)
import Update.Validate exposing (validateActivityForm)


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        GotActivities result ->
            case result of
                Ok items ->
                    ( { model | activities = Success items }, Cmd.none )

                Err message ->
                    ( { model | activities = Failure message }, Cmd.none )

        OpenedActivityForm ->
            ( { model
                | activityForm = Just emptyActivityForm
                , toast = Nothing
              }
            , Cmd.none
            )

        RequestedCloseActivityForm ->
            case ( model.activityForm, model.deletingActivity ) of
                ( _, Just _ ) ->
                    ( { model | deletingActivity = Nothing }, Cmd.none )

                ( Just af, _ ) ->
                    if af.dirty && not af.submitting then
                        ( { model | activityForm = Just { af | confirmDiscard = True } }, Cmd.none )

                    else
                        ( { model | activityForm = Nothing }, Cmd.none )

                _ ->
                    ( model, Cmd.none )

        ConfirmedCloseActivityForm ->
            ( { model | activityForm = Nothing }, Cmd.none )

        CancelledCloseActivityForm ->
            case model.activityForm of
                Just af ->
                    ( { model | activityForm = Just { af | confirmDiscard = False } }, Cmd.none )

                Nothing ->
                    ( model, Cmd.none )

        UpdatedActivityFormField field value ->
            case model.activityForm of
                Just af ->
                    let
                        updated =
                            case field of
                                "kind" ->
                                    { af | kind = value }

                                "title" ->
                                    { af | title = value }

                                "body" ->
                                    { af | body = value }

                                "occurredAt" ->
                                    { af | occurredAt = value }

                                _ ->
                                    af
                    in
                    ( { model
                        | activityForm =
                            Just
                                { updated
                                    | dirty = True
                                    , errors =
                                        List.filter (\( f, _ ) -> f /= field) updated.errors
                                }
                      }
                    , Cmd.none
                    )

                Nothing ->
                    ( model, Cmd.none )

        SubmittedActivityForm ->
            case ( model.activityForm, model.token, model.viewingContact ) of
                ( Just af, Just token, Just contact ) ->
                    let
                        ( validated, ok ) =
                            validateActivityForm af
                    in
                    if not ok then
                        ( { model | activityForm = Just validated }, Cmd.none )

                    else
                        ( { model
                            | activityForm = Just { validated | submitting = True }
                          }
                        , Api.createActivity token contact.id validated GotSavedActivity
                        )

                _ ->
                    ( model, Cmd.none )

        GotSavedActivity result ->
            case result of
                Ok activity ->
                    let
                        freshModel =
                            { model
                                | activityForm = Nothing
                                , toast = Just ("Logged " ++ activity.title)
                            }

                        contactId =
                            case model.viewingContact of
                                Just c ->
                                    c.id

                                Nothing ->
                                    ""
                    in
                    if String.isEmpty contactId then
                        ( freshModel, Cmd.none )

                    else
                        ( freshModel
                        , Cmd.batch
                            [ Tuple.second (loadActivities freshModel contactId)
                            , Tuple.second (loadContacts freshModel)
                            ]
                        )

                Err (FieldErrors fields) ->
                    case model.activityForm of
                        Just af ->
                            ( { model
                                | activityForm =
                                    Just { af | submitting = False, errors = fields }
                              }
                            , Cmd.none
                            )

                        Nothing ->
                            ( model, Cmd.none )

                Err (GenericError message) ->
                    case model.activityForm of
                        Just af ->
                            ( { model
                                | activityForm =
                                    Just
                                        { af
                                            | submitting = False
                                            , errors = [ ( "form", message ) ]
                                        }
                              }
                            , Cmd.none
                            )

                        Nothing ->
                            ( model, Cmd.none )

        RequestedDeleteActivity activity ->
            ( { model | deletingActivity = Just activity }, Cmd.none )

        CancelledDeleteActivity ->
            ( { model | deletingActivity = Nothing }, Cmd.none )

        ConfirmedDeleteActivity ->
            case ( model.deletingActivity, model.token ) of
                ( Just activity, Just token ) ->
                    ( model, Api.deleteActivity token activity.id activity.revision GotDeletedActivity )

                _ ->
                    ( model, Cmd.none )

        GotDeletedActivity result ->
            case result of
                Ok _ ->
                    let
                        freshModel =
                            { model
                                | deletingActivity = Nothing
                                , toast = Just "Activity deleted"
                            }

                        contactId =
                            case model.viewingContact of
                                Just c ->
                                    c.id

                                Nothing ->
                                    ""
                    in
                    if String.isEmpty contactId then
                        ( freshModel, Cmd.none )

                    else
                        ( freshModel, Tuple.second (loadActivities freshModel contactId) )

                Err message ->
                    ( { model
                        | deletingActivity = Nothing
                        , toast = Just ("Delete failed: " ++ message)
                      }
                    , Cmd.none
                    )

        _ ->
            ( model, Cmd.none )
