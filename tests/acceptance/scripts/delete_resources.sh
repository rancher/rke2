#!/bin/bash

#Get resource name from tfvarslocal && change name to make more sense in this context
RESOURCE_NAME=$(grep resource_name <modules/config/local.tfvars | cut -d= -f2 | tr -d ' "')
name_prefix="$RESOURCE_NAME"


##Terminate the instances
echo "Terminating resources for $name_prefix if still up and running"
# shellcheck disable=SC2046
aws ec2 terminate-instances --instance-ids $(aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=${name_prefix}*" \
  "Name=instance-state-name,Values=running" --query \
  'Reservations[].Instances[].InstanceId' --output text) > /dev/null 2>&1


#Get the list of load balancer ARNs
lb_arn_list=$(aws elbv2 describe-load-balancers \
  --query "LoadBalancers[?starts_with(LoadBalancerName, '${name_prefix}') && Type=='network'].LoadBalancerArn" \
  --output text)


#Loop through the load balancer ARNs and delete the load balancers
for lb_arn in $lb_arn_list; do
  echo "Deleting load balancer $lb_arn"
  aws elbv2 delete-load-balancer --load-balancer-arn "$lb_arn"
done


#Get the list of target group ARNs
tg_arn_list=$(aws elbv2 describe-target-groups \
  --query "TargetGroups[?starts_with(TargetGroupName, '${name_prefix}') && Protocol=='TCP'].TargetGroupArn" \
  --output text)


#Loop through the target group ARNs and delete the target groups
for tg_arn in $tg_arn_list; do
  echo "Deleting target group $tg_arn"
  aws elbv2 delete-target-group --target-group-arn "$tg_arn"
done


#Get the ID and recordName with lower case of the hosted zone that contains the Route 53 record sets
name_prefix_lower=$(echo "$name_prefix" | tr '[:upper:]' '[:lower:]')
r53_zone_id=$(aws route53 list-hosted-zones-by-name --dns-name "${name_prefix}." \
  --query "HostedZones[0].Id" --output text)
r53_record=$(aws route53 list-resource-record-sets \
  --hosted-zone-id "${r53_zone_id}" \
  --query "ResourceRecordSets[?starts_with(Name, '${name_prefix_lower}.') && Type == 'CNAME'].Name" \
  --output text)


#Get ResourceRecord Value
record_value=$(aws route53 list-resource-record-sets \
  --hosted-zone-id "${r53_zone_id}" \
  --query "ResourceRecordSets[?starts_with(Name, '${name_prefix_lower}.') \
    && Type == 'CNAME'].ResourceRecords[0].Value" --output text)


#Delete Route53 record
if [[ "$r53_record" == "${name_prefix_lower}."* ]]; then
  echo "Deleting Route53 record ${r53_record}"
  change_status=$(aws route53 change-resource-record-sets --hosted-zone-id "${r53_zone_id}" \
    --change-batch '{"Changes": [
            {
                "Action": "DELETE",
                "ResourceRecordSet": {
                    "Name": "'"${r53_record}"'",
                    "Type": "CNAME",
                    "TTL": 300,
                    "ResourceRecords": [
                        {
                            "Value": "'"${record_value}"'"
                        }
                    ]
                }
            }
        ]
    }')
  status_id=$(echo "$change_status" | jq -r '.ChangeInfo.Id')
  #Get status from the change
  aws route53 wait resource-record-sets-changed --id "$status_id"
  echo "Successfully deleted Route53 record ${r53_record}: status: ${status_id}"
else
  echo "No Route53 record found"
fi