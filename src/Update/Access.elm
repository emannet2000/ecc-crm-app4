module Update.Access exposing (allowed, canCreate, busy)

import Types exposing (..)


canCreate : Model -> Bool
canCreate model =
    model.user |> Maybe.map (\user -> List.member user.role [ "admin", "manager", "member" ]) |> Maybe.withDefault False


allowed : Msg -> Model -> Bool
allowed msg model =
    case msg of
        OpenedEditContact record ->
            record.editable

        RequestedDeleteContact record ->
            record.editable

        OpenedEditDeal record ->
            record.editable

        MovedDeal record _ ->
            record.editable

        RequestedDeleteDeal record ->
            record.editable

        RequestedDeleteActivity record ->
            record.editable

        OpenedEditTask record ->
            record.editable

        ToggledTaskStatus record _ ->
            record.editable

        RequestedDeleteTask record ->
            record.editable

        OpenedEditSchool record ->
            record.editable

        RequestedDeleteSchool record ->
            record.editable

        OpenedEditStudent record ->
            record.editable

        RequestedDeleteStudent record ->
            record.editable

        OpenedEditAgent record ->
            record.editable

        RequestedDeleteAgent record ->
            record.editable

        OpenedEditLead record ->
            record.editable

        RequestedDeleteLead record ->
            record.editable

        OpenedEditCase record ->
            record.editable

        RequestedDeleteCase record ->
            record.editable

        OpenedEditDocument record ->
            record.editable

        RequestedDeleteDocument record ->
            record.editable

        UpdatedDocumentStatus record _ ->
            record.editable

        OpenedEditInvoice record ->
            record.editable

        RequestedDeleteInvoice record ->
            record.editable

        RequestedDeletePayment record ->
            record.editable

        OpenedEditPartner record ->
            record.editable

        RequestedDeletePartner record ->
            record.editable

        OpenedAddContact ->
            canCreate model

        SubmittedContactForm ->
            canCreate model

        ConfirmedDeleteContact ->
            canCreate model

        OpenedAddDeal ->
            canCreate model

        OpenedAddDealForContact _ ->
            canCreate model

        OpenedAddDealWithStage _ ->
            canCreate model

        SubmittedDealForm ->
            canCreate model

        ConfirmedDeleteDeal ->
            canCreate model

        DraggingDealStarted _ ->
            canCreate model

        DealDroppedOnStage _ ->
            canCreate model

        RequestedBulkMove _ ->
            canCreate model

        ConfirmedBulkMove ->
            canCreate model

        RequestedBulkDelete ->
            canCreate model

        ConfirmedBulkDelete ->
            canCreate model

        SubmittedActivityForm ->
            canCreate model

        ConfirmedDeleteActivity ->
            canCreate model

        OpenedAddTask ->
            canCreate model

        OpenedAddTaskForContact _ ->
            canCreate model

        SubmittedTaskForm ->
            canCreate model

        ConfirmedDeleteTask ->
            canCreate model

        OpenedAddSchool ->
            canCreate model

        SubmittedSchoolForm ->
            canCreate model

        ConfirmedDeleteSchool ->
            canCreate model

        OpenedAddStudent ->
            canCreate model

        OpenedAddStudentForAgent _ ->
            canCreate model

        OpenedAddStudentForSchool _ ->
            canCreate model

        SubmittedStudentForm ->
            canCreate model

        ConfirmedDeleteStudent ->
            canCreate model

        OpenedAddAgent ->
            canCreate model

        SubmittedAgentForm ->
            canCreate model

        ConfirmedDeleteAgent ->
            canCreate model

        OpenedAddLead ->
            canCreate model

        SubmittedLeadForm ->
            canCreate model

        ConfirmedDeleteLead ->
            canCreate model

        OpenedAddCase ->
            canCreate model

        OpenedAddCaseForStudent _ ->
            canCreate model

        SubmittedCaseForm ->
            canCreate model

        ConfirmedDeleteCase ->
            canCreate model

        OpenedAddDocument ->
            canCreate model

        SubmittedDocumentForm ->
            canCreate model

        ConfirmedDeleteDocument ->
            canCreate model

        OpenedAddInvoice ->
            canCreate model

        SubmittedInvoiceForm ->
            canCreate model

        ConfirmedDeleteInvoice ->
            canCreate model

        SubmittedPaymentForm ->
            canCreate model

        ConfirmedDeletePayment ->
            canCreate model

        RequestedRefund record ->
            record.editable

        ConfirmedRefund ->
            canCreate model

        OpenedAddPartner ->
            canCreate model

        SubmittedPartnerForm ->
            canCreate model

        ConfirmedDeletePartner ->
            canCreate model

        OpenedActivityForm ->
            canCreate model

        OpenedPaymentForm ->
            model.viewingInvoice |> Maybe.map .editable |> Maybe.withDefault False

        _ ->
            True


busy : Msg -> Model -> Bool
busy msg model =
    case msg of
        Submitted -> model.submitting
        SubmittedContactForm -> Maybe.map .submitting model.contactForm |> Maybe.withDefault False
        RequestedCloseContactForm -> Maybe.map .submitting model.contactForm |> Maybe.withDefault False
        ConfirmedCloseContactForm -> Maybe.map .submitting model.contactForm |> Maybe.withDefault False
        SubmittedDealForm -> Maybe.map .submitting model.dealForm |> Maybe.withDefault False
        RequestedCloseDealForm -> Maybe.map .submitting model.dealForm |> Maybe.withDefault False
        ConfirmedCloseDealForm -> Maybe.map .submitting model.dealForm |> Maybe.withDefault False
        SubmittedTaskForm -> Maybe.map .submitting model.taskForm |> Maybe.withDefault False
        RequestedCloseTaskForm -> Maybe.map .submitting model.taskForm |> Maybe.withDefault False
        ConfirmedCloseTaskForm -> Maybe.map .submitting model.taskForm |> Maybe.withDefault False
        SubmittedSchoolForm -> Maybe.map .submitting model.schoolForm |> Maybe.withDefault False
        RequestedCloseSchoolForm -> Maybe.map .submitting model.schoolForm |> Maybe.withDefault False
        ConfirmedCloseSchoolForm -> Maybe.map .submitting model.schoolForm |> Maybe.withDefault False
        SubmittedStudentForm -> Maybe.map .submitting model.studentForm |> Maybe.withDefault False
        RequestedCloseStudentForm -> Maybe.map .submitting model.studentForm |> Maybe.withDefault False
        ConfirmedCloseStudentForm -> Maybe.map .submitting model.studentForm |> Maybe.withDefault False
        SubmittedAgentForm -> Maybe.map .submitting model.agentForm |> Maybe.withDefault False
        RequestedCloseAgentForm -> Maybe.map .submitting model.agentForm |> Maybe.withDefault False
        ConfirmedCloseAgentForm -> Maybe.map .submitting model.agentForm |> Maybe.withDefault False
        SubmittedLeadForm -> Maybe.map .submitting model.leadForm |> Maybe.withDefault False
        RequestedCloseLeadForm -> Maybe.map .submitting model.leadForm |> Maybe.withDefault False
        ConfirmedCloseLeadForm -> Maybe.map .submitting model.leadForm |> Maybe.withDefault False
        SubmittedCaseForm -> Maybe.map .submitting model.caseForm |> Maybe.withDefault False
        RequestedCloseCaseForm -> Maybe.map .submitting model.caseForm |> Maybe.withDefault False
        ConfirmedCloseCaseForm -> Maybe.map .submitting model.caseForm |> Maybe.withDefault False
        SubmittedDocumentForm -> Maybe.map .submitting model.documentForm |> Maybe.withDefault False
        RequestedCloseDocumentForm -> Maybe.map .submitting model.documentForm |> Maybe.withDefault False
        ConfirmedCloseDocumentForm -> Maybe.map .submitting model.documentForm |> Maybe.withDefault False
        SubmittedInvoiceForm -> Maybe.map .submitting model.invoiceForm |> Maybe.withDefault False
        RequestedCloseInvoiceForm -> Maybe.map .submitting model.invoiceForm |> Maybe.withDefault False
        ConfirmedCloseInvoiceForm -> Maybe.map .submitting model.invoiceForm |> Maybe.withDefault False
        SubmittedPartnerForm -> Maybe.map .submitting model.partnerForm |> Maybe.withDefault False
        RequestedClosePartnerForm -> Maybe.map .submitting model.partnerForm |> Maybe.withDefault False
        ConfirmedClosePartnerForm -> Maybe.map .submitting model.partnerForm |> Maybe.withDefault False
        SubmittedActivityForm -> Maybe.map .submitting model.activityForm |> Maybe.withDefault False
        RequestedCloseActivityForm -> Maybe.map .submitting model.activityForm |> Maybe.withDefault False
        ConfirmedCloseActivityForm -> Maybe.map .submitting model.activityForm |> Maybe.withDefault False
        SubmittedPaymentForm -> Maybe.map .submitting model.paymentForm |> Maybe.withDefault False
        ClosedPaymentForm -> Maybe.map .submitting model.paymentForm |> Maybe.withDefault False
        _ -> False
